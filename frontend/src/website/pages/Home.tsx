import { HeroSection } from '../components/HeroSection'
import { CapabilitySection } from '../components/CapabilitySection'
import { SolutionSection } from '../components/SolutionSection'
import { ScenarioSection } from '../components/ScenarioSection'
import { StatsSection } from '../components/StatsSection'
import { ContactSection } from '../components/ContactSection'

/** 单页锚点顺序：首屏 → 能力 → 流程 → 场景 → 数据 → 联系 */
export function Home() {
  return (
    <>
      <HeroSection />
      <CapabilitySection />
      <SolutionSection />
      <ScenarioSection />
      <StatsSection />
      <ContactSection />
    </>
  )
}

export default Home
