"use client";
import {useState} from "react";
import {Clock, Flag, ListPlus, MonitorCog, Truck} from "lucide-react";
import {DropdownMenu} from "@/components/DropdownMenu";
import type {CatalogItem, Company} from "@/services/core";
import type {Order, OrderFilters} from "@/services/orders";
import {AssignWaveModal} from "./AssignWaveModal";
import {CourierSettingsModal} from "./CourierSettingsModal";
import {CutoffsModal} from "./CutoffsModal";
import {WaveRulesModal} from "./WaveRulesModal";

type Dialog = "rules" | "cutoffs" | "couriers" | "assign";

export function AssignmentActions({selected, filters, companyId, companies, couriers, onChanged, onAssigned}: {
    selected: Order[];
    filters: OrderFilters;
    companyId: number | null;
    companies: Company[];
    couriers: CatalogItem[];
    onChanged: () => void;
    onAssigned: () => void;
}) {
    const [dialog, setDialog] = useState<Dialog | null>(null);
    const close = () => setDialog(null);
    return <div className="assignment-heading">
    <h2>Órdenes por asignar</h2>
    <DropdownMenu className="assignment-config" menuClassName="assignment-menu" triggerClassName="btn assignment-config-trigger" trigger={<><MonitorCog size={20}/>Configurar asignaciones</>} items={[
        {label: "Configurar asignación", icon: <Flag size={18} aria-hidden="true"/>, onSelect: () => setDialog("rules")},
        {label: "Configurar cortes", icon: <Clock size={18} aria-hidden="true"/>, onSelect: () => setDialog("cutoffs")},
        {label: "Configurar courier", icon: <Truck size={18} aria-hidden="true"/>, onSelect: () => setDialog("couriers")},
    ]}/>
    <button className="btn assignment-wave" onClick={() => setDialog("assign")}>Asignar a ola de Invas <ListPlus size={20}/></button>
    {dialog === "rules" && <WaveRulesModal companies={companies} couriers={couriers} onClose={close} onChanged={onChanged}/>}
    {dialog === "cutoffs" && <CutoffsModal couriers={couriers} onClose={close} onChanged={onChanged}/>}
    {dialog === "couriers" && <CourierSettingsModal couriers={couriers} onClose={close} onChanged={onChanged} onOpenCutoffs={() => setDialog("cutoffs")}/>}
    {dialog === "assign" && <AssignWaveModal selected={selected} filters={filters} companyId={companyId} onClose={close} onDone={onAssigned}/>}
    </div>;
}
