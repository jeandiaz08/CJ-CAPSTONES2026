import { PackingOrder } from "@/features/packing";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PackingOrder id={Number(id)} />;
}
