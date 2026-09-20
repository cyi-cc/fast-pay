import { Client, type result, type RequestOptions } from "./client";
import type bindGoodsDto from "./bindGoodsDto";
import type testPayDto from "./testPayDto";
import type testPayResult from "./testPayResult";
import type upstreamAccountDto from "./upstreamAccountDto";
import type upstreamCardAddDto from "./upstreamCardAddDto";
import type upstreamCardPage from "./upstreamCardPage";
import type upstreamCardQueryDto from "./upstreamCardQueryDto";
import type upstreamGoodsPage from "./upstreamGoodsPage";
import type upstreamGoodsQueryDto from "./upstreamGoodsQueryDto";
import type upstreamStatusView from "./upstreamStatusView";
import type upstreamStockDto from "./upstreamStockDto";

export default class upstreamSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async bindGoods(dto:bindGoodsDto, options?: RequestOptions): Promise<result<upstreamStatusView>> {
    return await this.client.request<upstreamStatusView>("upstreamSvc", "bindGoods", dto, options)
  }
  async cardAdd(dto:upstreamCardAddDto, options?: RequestOptions): Promise<result<string>> {
    return await this.client.request<string>("upstreamSvc", "cardAdd", dto, options)
  }
  async cardList(dto:upstreamCardQueryDto, options?: RequestOptions): Promise<result<upstreamCardPage>> {
    return await this.client.request<upstreamCardPage>("upstreamSvc", "cardList", dto, options)
  }
  async goodsList(dto:upstreamGoodsQueryDto, options?: RequestOptions): Promise<result<upstreamGoodsPage>> {
    return await this.client.request<upstreamGoodsPage>("upstreamSvc", "goodsList", dto, options)
  }
  async saveAccount(dto:upstreamAccountDto, options?: RequestOptions): Promise<result<upstreamStatusView>> {
    return await this.client.request<upstreamStatusView>("upstreamSvc", "saveAccount", dto, options)
  }
  async setStock(dto:upstreamStockDto, options?: RequestOptions): Promise<result<upstreamStatusView>> {
    return await this.client.request<upstreamStatusView>("upstreamSvc", "setStock", dto, options)
  }
  async status(options?: RequestOptions): Promise<result<upstreamStatusView>> {
    return await this.client.request<upstreamStatusView>("upstreamSvc", "status", undefined, options)
  }
  async testPay(dto:testPayDto, options?: RequestOptions): Promise<result<testPayResult>> {
    return await this.client.request<testPayResult>("upstreamSvc", "testPay", dto, options)
  }
}