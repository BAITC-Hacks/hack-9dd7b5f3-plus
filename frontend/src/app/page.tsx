import { Band } from "@/components/landing/band";
import { EdgeClouds } from "@/components/landing/clouds";
import { Cta } from "@/components/landing/cta";
import { Footer } from "@/components/landing/footer";
import { Header } from "@/components/landing/header";
import { Hero, Horizon } from "@/components/landing/hero";
import { Product } from "@/components/landing/product";
import { Reveal } from "@/components/landing/reveal";
import { Scenarios } from "@/components/landing/scenarios";
import { TraceSection } from "@/components/landing/trace";

export default function Home() {
  return (
    <div className="relative flex min-h-screen flex-col overflow-x-clip bg-paper text-ink">
      <EdgeClouds />
      <Header />
      <main className="flex-1">
        <Hero />
        <Horizon />
        <Band id="console" title="Консоль" text="Живая трассировка каждой реплики: сценарий, уверенность, причина, альтернативы, действия и время по этапам." />
        <Reveal><Product /></Reveal>
        <Band id="scenarios" title="Каталог" text="Сорок сценариев страховой компании и три системных намерения. Роутер знает о них ровно то, что написано в каталоге." />
        <Reveal><Scenarios /></Reveal>
        <Band id="trace" title="Трассировка" text="Формат из стартового кита кейса. Один JSON на реплику — для супервизора, для оценки, для жюри." />
        <Reveal><TraceSection /></Reveal>
        <Reveal><Cta /></Reveal>
      </main>
      <Footer />
    </div>
  );
}
