import { Client, type result, type RequestOptions } from "./client";
import type cashierStatus from "./cashierStatus";
import type cashierView from "./cashierView";
import type tradeNoDto from "./tradeNoDto";

export default class cashierSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async confirm(dto:tradeNoDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("cashierSvc", "confirm", dto, options)
  }
  async get(dto:tradeNoDto, options?: RequestOptions): Promise<result<cashierView>> {
    return await this.client.request<cashierView>("cashierSvc", "get", dto, options)
  }
  async status(dto:tradeNoDto, options?: RequestOptions): Promise<result<cashierStatus>> {
    return await this.client.request<cashierStatus>("cashierSvc", "status", dto, options)
  }
}