import type adminOrderView from "./adminOrderView";
export default interface dashboardView {
  todayCount:number
  todayMoney:number
  todayFee:number
  userCount:number
  paidCount:number
  totalMoney:number
  pendingSettle:number
  recent:adminOrderView[]
}