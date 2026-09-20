// 大整数安全 JSON：超过 Number.MAX_SAFE_INTEGER（2^53-1）的整数字面量
// 解析为 BigInt，序列化时 BigInt 还原为数字字面量——雪花 ID 等不再丢精度。
// 自包含实现，不引入运行时依赖；如需换 json-bigint，替换下面两个函数即可
const BIGINT_PREFIX = "\u0000fun-bigint:";

function reviveBigint(value: any): any {
  if (typeof value === "string" && value.startsWith(BIGINT_PREFIX) &&
      /^-?\d+$/.test(value.slice(BIGINT_PREFIX.length))) {
    return BigInt(value.slice(BIGINT_PREFIX.length));
  }
  if (Array.isArray(value)) return value.map(reviveBigint);
  if (value !== null && typeof value === "object") {
    for (const key of Object.keys(value)) value[key] = reviveBigint(value[key]);
    return value;
  }
  return value;
}

function parseLossless(text: string): any {
  // 先把超出安全范围的大整数包成带哨兵的字符串（正则的字符串分支优先，
  // 字符串内容里的数字不受影响），JSON.parse 后再还原为 BigInt
  const guarded = text.replace(
    /"(?:[^"\\]|\\(?:["\\\/bfnrt]|u[0-9a-fA-F]{4}))*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g,
    token => {
      if (token.startsWith('"')) return token;
      if (!/[.eE]/.test(token) && !Number.isSafeInteger(Number(token))) {
        // 哨兵必须以转义形式 \\u0000 写入文本（JSON 字符串禁止裸控制字符），
        // JSON.parse 解码后即为 BIGINT_PREFIX（真实 NUL 开头）
        return '"\\u0000fun-bigint:' + token + '"';
      }
      return token;
    }
  );
  return reviveBigint(JSON.parse(guarded));
}

function stringifyLossless(value: any): string | undefined {
  const raw = JSON.stringify(value, (_key, v) =>
    typeof v === "bigint" ? BIGINT_PREFIX + v.toString() : v
  );
  if (raw === undefined) return undefined;
  // 哨兵字符串在序列化文本中形如 "\u0000fun-bigint:123"（NUL 被转义），去引号还原为数字字面量
  return raw.replace(/"\\u0000fun-bigint:(-?\d+)"/g, "$1");
}

export type result<T> = {
  code?: number;
  data?: T;
  msg?: string;
  status: number;
};

export type resultStatus = 0 | 1 | 2 | 4 | 5;
// 0 success; 1 framework/client protocol error; 2 business error; 4 external request error; 5 external timeout

export type RequestOptions = {
  signal?: AbortSignal;
  state?: Record<string, string>;
};

export type StreamOptions = {
  signal?: AbortSignal;
  state?: Record<string, string>;
  /**
   * 原生断线重连：流因传输失败（status 4/5）结束时按指数退避自动重连。
   * dto 传工厂函数时每次重连重新求值，用于刷新续传游标。
   * 未传时关闭（默认），与 request 语义一致。
   */
  retry?: StreamRetryOptions | false;
  /** 空闲看门狗：超过该毫秒数未收到任何字节（含心跳空行）即判定流死亡并走重连。0 关闭。 */
  idleTimeoutMs?: number;
};

export type StreamRetryOptions = {
  /** 最大重连次数，默认 Infinity（直到 signal 取消或成功）。 */
  maxAttempts?: number;
  /** 首次重连延迟（毫秒），默认 500。 */
  baseDelayMs?: number;
  /** 退避上限（毫秒），默认 10000。实际延迟为指数退避 × 0.75~1.25 抖动。 */
  maxDelayMs?: number;
};

export type RequestInterceptor = (
  serviceName: string,
  methodName: string,
  state: Record<string, string>,
  dto?: any
) => Promise<void> | void;

export type ResponseContext = {
  readonly requestState: Readonly<Record<string, string>>;
  readonly response?: Response;
};

export type ResponseInterceptor = (
  serviceName: string,
  methodName: string,
  result: result<any>
) => Promise<result<any> | void> | result<any> | void;

export type ContextResponseInterceptor = (
  serviceName: string,
  methodName: string,
  result: result<any>,
  context: ResponseContext
) => Promise<result<any> | void> | result<any> | void;

function messageOf(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === "string" && error) return error;
  return "unknown error";
}

function failure(status: resultStatus, msg: string): result<any> {
  return { status, msg };
}

function isTimeout(error: unknown, signal?: AbortSignal): boolean {
  const errorName = error !== null && typeof error === "object"
    ? (error as { name?: unknown }).name
    : undefined;
  const reason = signal?.reason;
  const reasonName = reason !== null && typeof reason === "object"
    ? (reason as { name?: unknown }).name
    : undefined;
  return errorName === "TimeoutError" || reasonName === "TimeoutError";
}

function requestFailure(error: unknown, signal: AbortSignal | undefined, stream: boolean): result<any> {
  if (isTimeout(error, signal)) {
    return failure(5, stream ? "Stream timed out" : "Request timed out");
  }
  if (signal?.aborted === true) {
    return failure(4, stream ? "Stream aborted" : "Request aborted");
  }
  const kind = stream ? "External stream request" : "External request";
  return failure(4, `${kind} failed: ${messageOf(error)}`);
}

function isResult(value: unknown): value is result<any> {
  return value !== null && typeof value === "object" &&
    typeof (value as { status?: unknown }).status === "number";
}

function mediaType(response: Response): string {
  return (response.headers.get("content-type") || "").split(";", 1)[0].trim().toLowerCase();
}

function excerpt(text: string, limit = 180): string {
  const value = text.replace(/\s+/g, " ").trim();
  return value.length <= limit ? value : `${value.slice(0, limit)}...`;
}

function externalFailure(response: Response, detail?: string): result<any> {
  const timeout = response.status === 408 || response.status === 504;
  const statusText = response.statusText || (timeout ? "timeout" : "request failed");
  const suffix = detail ? `: ${detail}` : "";
  return failure(timeout ? 5 : 4, `HTTP ${response.status} ${statusText}${suffix}`);
}

function responseReadFailure(
  error: unknown,
  response: Response,
  signal: AbortSignal | undefined,
  stream: boolean
): result<any> {
  if (isTimeout(error, signal)) {
    return failure(5, stream ? "Stream timed out" : "Request timed out");
  }
  if (signal?.aborted === true) {
    return failure(4, stream ? "Stream aborted" : "Request aborted");
  }
  const kind = stream ? "Stream" : "Response body";
  return response.ok
    ? failure(4, `${kind} failed: ${messageOf(error)}`)
    : externalFailure(response, `response body failed: ${messageOf(error)}`);
}

function parseResult(response: Response, text: string): result<any> {
  const body = text.trim();
  if (!body) {
    return response.ok
      ? failure(1, "Empty response body")
      : externalFailure(response);
  }

  let value: unknown;
  try {
    value = parseLossless(body);
  } catch {
    if (!response.ok) return externalFailure(response, excerpt(body));
    const type = mediaType(response);
    if (type === "text/html" || /^\s*(?:<!doctype\s+html|<html\b)/i.test(body)) {
      return failure(1, `Unexpected HTML response: ${excerpt(body)}`);
    }
    return failure(1, `Invalid JSON response: ${excerpt(body)}`);
  }
  if (!isResult(value)) {
    return response.ok
      ? failure(1, "Invalid fun response")
      : externalFailure(response, "invalid fun response");
  }
  return value;
}

export class Client {
  private url: string;
  private state: Record<string, string> = {};
  private requestInterceptors: RequestInterceptor[] = [];
  private responseInterceptors: ContextResponseInterceptor[] = [];

  constructor(url: string) {
    this.url = url.replace(/\/+$/, "");
  }

  setState(state: Record<string, string>) {
    this.state = state;
  }

  addRequestInterceptor(interceptor: RequestInterceptor) {
    this.requestInterceptors.push(interceptor);
  }

  addResponseInterceptor(interceptor: ResponseInterceptor): void;
  addResponseInterceptor(interceptor: ContextResponseInterceptor): void;
  addResponseInterceptor(interceptor: ResponseInterceptor | ContextResponseInterceptor) {
    this.responseInterceptors.push(interceptor as ContextResponseInterceptor);
  }

  private async interceptResponse(
    serviceName: string,
    methodName: string,
    initial: result<any>,
    requestState: Readonly<Record<string, string>>,
    response?: Response
  ): Promise<result<any>> {
    let current = initial;
    const context: ResponseContext = Object.freeze(
      response === undefined ? { requestState } : { requestState, response }
    );
    for (const interceptor of this.responseInterceptors) {
      try {
        const replaced = await interceptor(serviceName, methodName, current, context);
        if (replaced) current = replaced;
      } catch (error) {
        current = failure(1, `Response interceptor failed: ${messageOf(error)}`);
      }
    }
    return current;
  }

  private async interceptRequest(
    serviceName: string,
    methodName: string,
    state: Record<string, string>,
    dto: any
  ): Promise<void> {
    for (const interceptor of this.requestInterceptors) {
      await interceptor(serviceName, methodName, state, dto);
    }
  }

  private snapshotState(state: Record<string, string>): Readonly<Record<string, string>> {
    return Object.freeze({ ...state });
  }

  async request<T>(
    serviceName: string,
    methodName: string,
    dto?: any,
    options?: RequestOptions
  ): Promise<result<T>> {
    let state: Record<string, string>;
    try {
      state = { ...this.state, ...options?.state };
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not prepare request state: ${messageOf(error)}`),
        Object.freeze({})
      ) as result<T>;
    }
    try {
      await this.interceptRequest(serviceName, methodName, state, dto);
    } catch (error) {
      let requestState: Readonly<Record<string, string>>;
      try {
        requestState = this.snapshotState(state);
      } catch (snapshotError) {
        return await this.interceptResponse(
          serviceName,
          methodName,
          failure(1, `Could not snapshot request state: ${messageOf(snapshotError)}`),
          Object.freeze({})
        ) as result<T>;
      }
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Request interceptor failed: ${messageOf(error)}`),
        requestState
      ) as result<T>;
    }
    let requestState: Readonly<Record<string, string>>;
    try {
      requestState = this.snapshotState(state);
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not snapshot request state: ${messageOf(error)}`),
        Object.freeze({})
      ) as result<T>;
    }

    let body: string;
    try {
      const serialized = stringifyLossless({
        serviceName,
        methodName,
        data: dto,
        ...(Object.keys(requestState).length ? { state: requestState } : {}),
      });
      if (serialized === undefined) throw new Error("serialization produced no output");
      body = serialized;
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not serialize request: ${messageOf(error)}`),
        requestState
      ) as result<T>;
    }

    let output: result<any>;
    let response: Response | undefined;
    try {
      response = await fetch(`${this.url}/cell`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        signal: options?.signal,
      });
      try {
        output = parseResult(response, await response.text());
      } catch (error) {
        output = responseReadFailure(error, response, options?.signal, false);
      }
    } catch (error) {
      output = requestFailure(error, options?.signal, false);
    }
    return await this.interceptResponse(serviceName, methodName, output, requestState, response) as result<T>;
  }

  async stream<T>(
    serviceName: string,
    methodName: string,
    dto: any | undefined | (() => any | undefined),
    onMessage: (data: T) => unknown,
    options?: StreamOptions
  ): Promise<result<void>> {
    // dto 工厂：重连时重新求值，用于刷新续传游标（如 afterId）
    const getDto = typeof dto === "function" ? (dto as () => any | undefined) : () => dto;
    // 重连默认关闭（与 request 语义一致）；传 retry: {...} 显式开启
    const retry = options?.retry
      ? {
          maxAttempts: Infinity,
          baseDelayMs: 500,
          maxDelayMs: 10000,
          ...options.retry,
        }
      : null;
    for (let attempt = 0; ; attempt++) {
      const r = await this.streamOnce<T>(serviceName, methodName, getDto(), onMessage, options);
      // 0 成功、1 协议错误、2 业务错误：终结态，不重连
      if (r.status !== 4 && r.status !== 5) return r;
      if (!retry) return r;
      if (options?.signal?.aborted) return r;
      if (attempt + 1 > retry.maxAttempts) return r;
      const delay =
        Math.min(retry.maxDelayMs, retry.baseDelayMs * 2 ** attempt) * (0.75 + Math.random() * 0.5);
      // 退避等待：AbortSignal.timeout 免定时器 API，且后台标签页照常走时
      await new Promise<void>((resolve) => {
        if (options?.signal?.aborted) {
          resolve();
          return;
        }
        AbortSignal.timeout(delay).addEventListener("abort", () => resolve(), { once: true });
        options?.signal?.addEventListener("abort", () => resolve(), { once: true });
      });
      if (options?.signal?.aborted) return r;
    }
  }

  private async streamOnce<T>(
    serviceName: string,
    methodName: string,
    dto: any | undefined,
    onMessage: (data: T) => unknown,
    options?: StreamOptions
  ): Promise<result<void>> {
    let state: Record<string, string>;
    try {
      state = { ...this.state, ...options?.state };
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not prepare request state: ${messageOf(error)}`),
        Object.freeze({})
      );
    }
    try {
      await this.interceptRequest(serviceName, methodName, state, dto);
    } catch (error) {
      let requestState: Readonly<Record<string, string>>;
      try {
        requestState = this.snapshotState(state);
      } catch (snapshotError) {
        return await this.interceptResponse(
          serviceName,
          methodName,
          failure(1, `Could not snapshot request state: ${messageOf(snapshotError)}`),
          Object.freeze({})
        );
      }
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Request interceptor failed: ${messageOf(error)}`),
        requestState
      );
    }
    let requestState: Readonly<Record<string, string>>;
    try {
      requestState = this.snapshotState(state);
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not snapshot request state: ${messageOf(error)}`),
        Object.freeze({})
      );
    }

    let body: string;
    try {
      const serialized = stringifyLossless({
        serviceName,
        methodName,
        data: dto,
        ...(Object.keys(requestState).length ? { state: requestState } : {}),
      });
      if (serialized === undefined) throw new Error("serialization produced no output");
      body = serialized;
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        failure(1, `Could not serialize request: ${messageOf(error)}`),
        requestState
      );
    }

    let response: Response;
    try {
      response = await fetch(`${this.url}/cell`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        signal: options?.signal,
      });
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        requestFailure(error, options?.signal, true),
        requestState
      );
    }

    if (!response.ok) {
      let text: string;
      try {
        text = await response.text();
      } catch (error) {
        return await this.interceptResponse(
          serviceName,
          methodName,
          responseReadFailure(error, response, options?.signal, true),
          requestState,
          response
        );
      }
      return await this.interceptResponse(
        serviceName,
        methodName,
        parseResult(response, text),
        requestState,
        response
      );
    }

    if (mediaType(response) !== "application/x-ndjson") {
      let text: string;
      try {
        text = await response.text();
      } catch (error) {
        return await this.interceptResponse(
          serviceName,
          methodName,
          responseReadFailure(error, response, options?.signal, true),
          requestState,
          response
        );
      }
      const rpcResult = parseResult(response, text);
      return await this.interceptResponse(
        serviceName,
        methodName,
        rpcResult.status === 0
          ? failure(1, "Expected application/x-ndjson response")
          : rpcResult,
        requestState,
        response
      );
    }

    if (!response.body) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        { status: 0 },
        requestState,
        response
      );
    }

    let reader: ReadableStreamDefaultReader<Uint8Array>;
    try {
      reader = response.body.getReader();
    } catch (error) {
      return await this.interceptResponse(
        serviceName,
        methodName,
        responseReadFailure(error, response, options?.signal, true),
        requestState,
        response
      );
    }
    const decoder = new TextDecoder("utf-8", { fatal: true });
    let buffer = "";
    let lineNumber = 0;
    let failed: result<any> | undefined;
    let cause: unknown;

    const emitLine = async (line: string) => {
      const payload = line.replace(/\r$/, "").trim();
      if (!payload) return;
      let data: T;
      try {
        data = parseLossless(payload) as T;
      } catch (error) {
        failed = failure(1, `Invalid NDJSON at line ${lineNumber}: ${excerpt(payload)}`);
        cause = error;
        return;
      }
      try {
        await onMessage(data);
      } catch (error) {
        failed = failure(1, `Stream callback failed: ${messageOf(error)}`);
        cause = error;
      }
    };

    try {
      // 空闲看门狗：idleTimeoutMs 内未收到任何字节（含心跳空行）判定流死亡，
      // 抛出哨兵错误 → status 5 → 交给上层重连。服务端 25s 心跳时建议 35000。
      const STREAM_IDLE = Symbol("stream-idle");
      for (;;) {
        let part: ReadableStreamReadResult<Uint8Array>;
        try {
          if (options?.idleTimeoutMs) {
            const idlePromise = new Promise<never>((_, reject) => {
              AbortSignal.timeout(options.idleTimeoutMs!).addEventListener(
                "abort",
                () => reject(STREAM_IDLE),
                { once: true }
              );
            });
            idlePromise.catch(() => {}); // read 先返回时防 unhandled rejection
            part = await Promise.race([reader.read(), idlePromise]);
          } else {
            part = await reader.read();
          }
        } catch (error) {
          cause = error;
          if (error === STREAM_IDLE) {
            failed = failure(5, "Stream idle timeout");
          } else if (isTimeout(error, options?.signal)) {
            failed = failure(5, "Stream timed out");
          } else if (options?.signal?.aborted === true) {
            failed = failure(4, "Stream aborted");
          } else {
            failed = failure(4, `Stream read failed: ${messageOf(error)}`);
          }
          break;
        }
        if (part.done) break;
        try {
          buffer += decoder.decode(part.value, { stream: true });
        } catch (error) {
          failed = failure(1, `Invalid UTF-8 stream data: ${messageOf(error)}`);
          cause = error;
          break;
        }
        for (;;) {
          const newline = buffer.indexOf("\n");
          if (newline < 0) break;
          const line = buffer.slice(0, newline);
          buffer = buffer.slice(newline + 1);
          lineNumber++;
          await emitLine(line);
          if (failed) break;
        }
        if (failed) break;
      }

      if (!failed) {
        try {
          buffer += decoder.decode();
        } catch (error) {
          failed = failure(1, `Invalid UTF-8 stream data: ${messageOf(error)}`);
          cause = error;
        }
      }
      if (!failed && buffer.length > 0) {
        lineNumber++;
        await emitLine(buffer);
      }

    } finally {
      if (failed) {
        try {
          await reader.cancel(cause);
        } catch {
          // The reader may already be closed by the runtime.
        }
      }
      try {
        reader.releaseLock();
      } catch {
        // The reader may already be errored or released.
      }
    }
    return await this.interceptResponse(
      serviceName,
      methodName,
      failed || { status: 0 },
      requestState,
      response
    );
  }
}