import { Client, type result, type RequestOptions } from "./client";
import type loginDto from "./loginDto";
import type loginResult from "./loginResult";
import type refreshResult from "./refreshResult";
import type registerDto from "./registerDto";
import type userView from "./userView";

export default class authSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async login(dto:loginDto, options?: RequestOptions): Promise<result<loginResult>> {
    return await this.client.request<loginResult>("authSvc", "login", dto, options)
  }
  async logout(options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("authSvc", "logout", undefined, options)
  }
  async me(options?: RequestOptions): Promise<result<userView>> {
    return await this.client.request<userView>("authSvc", "me", undefined, options)
  }
  async refresh(options?: RequestOptions): Promise<result<refreshResult>> {
    return await this.client.request<refreshResult>("authSvc", "refresh", undefined, options)
  }
  async register(dto:registerDto, options?: RequestOptions): Promise<result<loginResult>> {
    return await this.client.request<loginResult>("authSvc", "register", dto, options)
  }
}