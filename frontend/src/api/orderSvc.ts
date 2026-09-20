import { Client, type result, type RequestOptions } from "./client";
import type listOrdersDto from "./listOrdersDto";
import type orderPage from "./orderPage";
import type orderView from "./orderView";
import type recentDto from "./recentDto";
import type statsView from "./statsView";

export default class orderSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async list(dto:listOrdersDto, options?: RequestOptions): Promise<result<orderPage>> {
    return await this.client.request<orderPage>("orderSvc", "list", dto, options)
  }
  async recent(dto:recentDto, options?: RequestOptions): Promise<result<orderView[]>> {
    return await this.client.request<orderView[]>("orderSvc", "recent", dto, options)
  }
  async stats(options?: RequestOptions): Promise<result<statsView>> {
    return await this.client.request<statsView>("orderSvc", "stats", undefined, options)
  }
}