import Link from "next/link";
export function Tabs({ items }: {
    items: {
        href: string;
        label: string;
        active: boolean;
    }[];
}) {
    return <nav className="view-tabs" aria-label="Vistas de órdenes">{items.map(i => <Link key={i.href} href={i.href} aria-current={i.active ? "page" : undefined}>{i.label}</Link>)}</nav>;
}
