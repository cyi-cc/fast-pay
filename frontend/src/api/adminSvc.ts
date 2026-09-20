import { Client, type result, type RequestOptions } from "./client";
import type adjustBalanceDto from "./adjustBalanceDto";
import type adminOrderPage from "./adminOrderPage";
import type adminSettlePage from "./adminSettlePage";
import type adminUserPage from "./adminUserPage";
import type dashboardView from "./dashboardView";
import type handleSettleDto from "./handleSettleDto";
import type listOrdersDto from "./listOrdersDto";
import type listSettlesDto from "./listSettlesDto";
import type listUsersDto from "./listUsersDto";
import type saveSettingsDto from "./saveSettingsDto";
import type setUserStatusDto from "./setUserStatusDto";
import type settingItem from "./settingItem";
import type tradeNoAdminDto from "./tradeNoAdminDto";

export default class adminSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async adjustBalance(dto:adjustBalanceDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "adjustBalance", dto, options)
  }
  async compense(dto:tradeNoAdminDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "compense", dto, options)
  }
  async dashboard(options?: RequestOptions): Promise<result<dashboardView>> {
    return await this.client.request<dashboardView>("adminSvc", "dashboard", undefined, options)
  }
  async getSettings(options?: RequestOptions): Promise<result<settingItem[]>> {
    return await this.client.request<settingItem[]>("adminSvc", "getSettings", undefined, options)
  }
  async handleSettle(dto:handleSettleDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "handleSettle", dto, options)
  }
  async orders(dto:listOrdersDto, options?: RequestOptions): Promise<result<adminOrderPage>> {
    return await this.client.request<adminOrderPage>("adminSvc", "orders", dto, options)
  }
  async refund(dto:tradeNoAdminDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "refund", dto, options)
  }
  async resendNotify(dto:tradeNoAdminDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "resendNotify", dto, options)
  }
  async saveSettings(dto:saveSettingsDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "saveSettings", dto, options)
  }
  async setUserStatus(dto:setUserStatusDto, options?: RequestOptions): Promise<result<void>> {
    return await this.client.request<void>("adminSvc", "setUserStatus", dto, options)
  }
  async settles(dto:listSettlesDto, options?: RequestOptions): Promise<result<adminSettlePage>> {
    return await this.client.request<adminSettlePage>("adminSvc", "settles", dto, options)
  }
  async users(dto:listUsersDto, options?: RequestOptions): Promise<result<adminUserPage>> {
    return await this.client.request<adminUserPage>("adminSvc", "users", dto, options)
  }
}