import { Client, type result, type RequestOptions } from "./client";
import type applySettleDto from "./applySettleDto";
import type balanceLogListResult from "./balanceLogListResult";
import type pageDto from "./pageDto";
import type settleListResult from "./settleListResult";

export default class settleSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async apply(dto:applySettleDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("settleSvc", "apply", dto, options)
  }
  async list(dto:pageDto, options?: RequestOptions): Promise<result<settleListResult>> {
    return await this.client.request<settleListResult>("settleSvc", "list", dto, options)
  }
  async logs(dto:pageDto, options?: RequestOptions): Promise<result<balanceLogListResult>> {
    return await this.client.request<balanceLogListResult>("settleSvc", "logs", dto, options)
  }
}