import type orderView from "./orderView";
export default interface adminOrderView {
  orderView:orderView
  merchant:string
  notifyUrl:string
  returnUrl:string
}