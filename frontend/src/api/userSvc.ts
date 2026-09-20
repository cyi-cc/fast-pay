import { Client, type result, type RequestOptions } from "./client";
import type changePasswordDto from "./changePasswordDto";

export default class userSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async changePassword(dto:changePasswordDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("userSvc", "changePassword", dto, options)
  }
}