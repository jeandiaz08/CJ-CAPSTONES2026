import type {Order} from "@/services/orders";
export function channelLabel(channel?:string){if(!channel)return "—";return ({mercadolibre:"Mercado Libre",shopify:"Shopify"} as Record<string,string>)[channel]??channel.charAt(0).toUpperCase()+channel.slice(1)}
const capitalize=(value:string)=>value.charAt(0).toUpperCase()+value.slice(1);
export const DISPATCH_LABELS:Record<string,string>={domicilio:"Despacho a domicilio",retiro_oficina:"Retiro en oficina del courier",express:"Express (mismo día)"};
export function dispatchLabel(dispatchType?:string){if(!dispatchType)return "—";return DISPATCH_LABELS[dispatchType]??capitalize(dispatchType.replaceAll("_"," "))}
export function warehouseLabel(warehouse?:string){return warehouse?capitalize(warehouse):"—"}
export function originLabel(order:Pick<Order,"source"|"channel">){return order.source==="manual"?"Orden manual":order.source==="bulk"?"Carga masiva":channelLabel(order.channel)}
