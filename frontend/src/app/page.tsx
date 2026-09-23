import { Cta } from "@/components/landing/cta";
import { Footer } from "@/components/landing/footer";
import { Header } from "@/components/landing/header";
import { Hero } from "@/components/landing/hero";
import { How } from "@/components/landing/how";
import { Product } from "@/components/landing/product";
import { Why } from "@/components/landing/why";

export default function Home() {
  return (
    <div className="flex min-h-screen flex-col bg-paper text-ink">
      <Header />
      <main className="flex-1">
        <Hero />
        <Product />
        <How />
        <Why />
        <Cta />
      </main>
      <Footer />
    </div>
  );
}
