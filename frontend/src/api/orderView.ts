export default interface orderView {
  id:number
  tradeNo:string
  outTradeNo:string
  channel:string
  subject:string
  money:number
  fee:number
  status:number
  notified:number
  notifyAttempts:number
  notifyUrl:string
  paidAt:number
  expiredAt:number
  createdAt:number
  appName:string
  pid:number
}