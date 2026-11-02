import type {Order} from "@/services/orders";
import {SITUATION_LABELS} from "../format";
const tones={en_cola:"queue",invas_agotado:"exhausted",fuera_corte:"cutoff",sin_regla:"cutoff"};
export function AssignmentSituation({value}:{value:Order["assignmentSituation"]}){if(!value)return null;const tone=tones[value];return tone?<span className={`badge assignment-situation situation-${tone}`}>{SITUATION_LABELS[value]}</span>:null}
