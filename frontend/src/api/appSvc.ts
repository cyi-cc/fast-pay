import { Client, type result, type RequestOptions } from "./client";
import type appView from "./appView";
import type createAppDto from "./createAppDto";
import type deleteAppDto from "./deleteAppDto";
import type resetKeyDto from "./resetKeyDto";
import type resetKeyResult from "./resetKeyResult";
import type updateAppDto from "./updateAppDto";

export default class appSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async create(dto:createAppDto, options?: RequestOptions): Promise<result<appView>> {
    return await this.client.request<appView>("appSvc", "create", dto, options)
  }
  async delete(dto:deleteAppDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("appSvc", "delete", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<appView[]>> {
    return await this.client.request<appView[]>("appSvc", "list", undefined, options)
  }
  async resetKey(dto:resetKeyDto, options?: RequestOptions): Promise<result<resetKeyResult>> {
    return await this.client.request<resetKeyResult>("appSvc", "resetKey", dto, options)
  }
  async update(dto:updateAppDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("appSvc", "update", dto, options)
  }
}