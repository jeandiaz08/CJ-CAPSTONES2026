"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { apiErrorMessage } from "@/services/api";
import { listRequests } from "@/services/clients";
import { listIntegrations } from "@/services/integrations";
import { getSummary, type OrderStatus } from "@/services/orders";
import type { HomeCard } from "../homeConfig";

const TONES = ["orange", "blue", "green"] as const;

type Data = { summary?: Partial<Record<OrderStatus, number>>; integrations?: { active: number; total: number }; requests?: number };

// Tarjetas del Home: cada número viene de un endpoint existente, ya filtrado por el servidor.
// `refresh` cambia cuando llega un aviso en vivo y vuelve a consultar.
export function HomeCards({ cards, companyId, refresh }: { cards: HomeCard[]; companyId: number | null; refresh: number }) {
  const [data, setData] = useState<Data>({});
  const [error, setError] = useState<string | null>(null);
  const kinds = [...new Set(cards.map((c) => c.source.kind))].join(",");

  const load = useCallback(async () => {
    const wanted = kinds.split(",");
    const failed = (e: unknown) => { setError(apiErrorMessage(e)); return undefined; };
    const [summary, integrations, requests] = await Promise.all([
      wanted.includes("orders") ? getSummary(companyId).then((r) => r.data, failed) : undefined,
      wanted.includes("integrations") ? listIntegrations().then((r) => ({ active: r.data.filter((i) => i.active).length, total: r.data.length }), failed) : undefined,
      wanted.includes("requests") ? listRequests({ limit: 1 }).then((r) => r.counts.pending, failed) : undefined,
    ]);
    setData({ summary, integrations, requests });
  }, [kinds, companyId]);

  useEffect(() => {
    setError(null);
    void load();
  }, [load, refresh]);

  const value = (card: HomeCard): string | undefined => {
    const s = card.source;
    if (s.kind === "orders") return data.summary && String(s.statuses.reduce((sum, st) => sum + (data.summary?.[st] ?? 0), 0));
    if (s.kind === "integrations") return data.integrations && `${data.integrations.active}/${data.integrations.total}`;
    return data.requests === undefined ? undefined : String(data.requests);
  };

  if (cards.length === 0) return null;
  return (
    <section aria-label="Resumen">
      {error && <p className="error" role="alert">No se pudo cargar el resumen. {error}</p>}
      <div className="home-cards">
        {cards.map((card, i) => {
          const Icon = card.icon;
          return (
            <Link key={card.key} href={card.href} className={`home-card home-${TONES[i % TONES.length]}`} data-card={card.key}>
              <span className="home-card-title"><Icon size={28} aria-hidden="true" />{card.title}</span>
              <span className="home-card-value"><strong>{value(card) ?? "—"}</strong><span>{card.caption}</span></span>
              <span className="home-card-link">{card.linkLabel} <ArrowRight size={16} aria-hidden="true" /></span>
            </Link>
          );
        })}
      </div>
    </section>
  );
}
