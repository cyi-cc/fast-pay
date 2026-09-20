export default interface listOrdersDto {
  page:number
  pageSize:number
  status?:number | null
  channel?:string | null
  kw?:string | null
  from?:number | null
  to?:number | null
}