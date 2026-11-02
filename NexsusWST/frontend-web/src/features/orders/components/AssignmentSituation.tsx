import type {Order} from "@/services/orders";
const situations={en_cola:{label:"En cola",tone:"queue"},invas_agotado:{label:"Invas agotó reintentos",tone:"exhausted"},fuera_corte:{label:"Fuera de corte",tone:"cutoff"},sin_regla:{label:"Sin regla que calce",tone:"cutoff"}};
export function AssignmentSituation({value}:{value:Order["assignmentSituation"]}){if(!value)return null;const situation=situations[value];return situation?<span className={`badge assignment-situation situation-${situation.tone}`}>{situation.label}</span>:null}
