import Link from "next/link";
import { ArrowRight, Mic } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Container } from "./section";

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      {/* Dotted grid flair (CSS): s2 dots on paper, fades out toward the bottom */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-[560px] opacity-70 [mask-image:linear-gradient(to_bottom,black_30%,transparent)]"
        style={{
          backgroundImage: "radial-gradient(var(--s3) 1px, transparent 1.2px)",
          backgroundSize: "24px 24px",
          backgroundPosition: "12px 12px",
        }}
      />
      <Container className="relative pt-20 pb-16 md:pt-28 md:pb-24">
        <span className="marker marker-dot">[ 01 / 05 ]&nbsp;&nbsp;Voice Router — Halyk Bank · HackAlem AI</span>
        <h1 className="h-display text-ink mt-6 max-w-4xl text-[2.75rem] leading-[1.02] md:text-[4rem] lg:text-[4.5rem]">
          Голосовой робот, который понимает с&nbsp;первой фразы
        </h1>
        <p className="text-body mt-6 max-w-2xl text-lg leading-relaxed">
          Bagyt заменяет encoder-классификатор интентов на LLM-слой маршрутизации: держит контекст диалога, переключает тему
          на лету, разбирает переход с русского на казахский внутри одной фразы и объясняет супервизору, почему выбран
          именно этот сценарий.
        </p>
        <div className="mt-10 flex flex-wrap items-center gap-3">
          <Button size="lg" render={<Link href="/call" />}>
            <Mic /> Поговорить с роботом
          </Button>
          <Button size="lg" variant="outline" render={<Link href="/admin" />}>
            Консоль супервизора <ArrowRight />
          </Button>
          <span className="marker ml-1">40 сценариев · ru/kk · mock</span>
        </div>
      </Container>
    </section>
  );
}
