import { SiteHeader } from "@/components/landing/site-header";
import { Hero } from "@/components/landing/hero";
import { ScenarioStream } from "@/components/landing/scenario-stream";
import { Pipeline } from "@/components/landing/pipeline";
import { WhyClassifier } from "@/components/landing/why-classifier";
import { TraceExample } from "@/components/landing/trace-example";
import { NumbersStrip } from "@/components/landing/numbers-strip";
import { SiteFooter } from "@/components/landing/site-footer";

export default function Home() {
  return (
    <main className="flex flex-1 flex-col bg-background text-foreground">
      <SiteHeader />
      <Hero />
      <ScenarioStream />
      <Pipeline />
      <WhyClassifier />
      <TraceExample />
      <NumbersStrip />
      <SiteFooter />
    </main>
  );
}
