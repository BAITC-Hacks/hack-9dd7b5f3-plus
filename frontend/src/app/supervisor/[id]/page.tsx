import type { Metadata } from "next";
import { SessionView } from "./session-view";

export const metadata: Metadata = { title: "Session" };

export default async function SessionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SessionView id={id} />;
}
