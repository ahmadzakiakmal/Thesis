import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { SystemOverview } from "@/components/system-overview"
import { Layer1Monitoring } from "@/components/layer1-monitoring"
import { Layer2Monitoring } from "@/components/layer2-monitoring"
import { ClientMetrics } from "@/components/client-metrics"
import { TransactionFlow } from "@/components/transaction-flow"

export default function DashboardPage() {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-10 border-b bg-background">
        <div className="container flex h-16 items-center px-4 sm:px-6 lg:px-8">
          <h1 className="text-lg font-semibold">2 Layer BFT Consensus Monitoring System</h1>
          <div className="ml-auto flex items-center space-x-4">
            <div className="flex items-center space-x-2">
              <div className="h-2 w-2 rounded-full bg-green-500"></div>
              <span className="text-sm text-muted-foreground">System Healthy</span>
            </div>
          </div>
        </div>
      </header>
      <main className="flex-1 space-y-4 p-4 sm:p-6 lg:p-8">
        <SystemOverview />
        <Tabs defaultValue="overview" className="space-y-4">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="layer1">Layer 1</TabsTrigger>
            <TabsTrigger value="layer2">Layer 2</TabsTrigger>
            <TabsTrigger value="clients">Clients</TabsTrigger>
          </TabsList>
          <TabsContent value="overview" className="space-y-4">
            <TransactionFlow />
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              <ClientMetrics />
              <Layer2Monitoring />
              <Layer1Monitoring />
            </div>
          </TabsContent>
          <TabsContent value="layer1" className="space-y-4">
            <Layer1Monitoring detailed />
          </TabsContent>
          <TabsContent value="layer2" className="space-y-4">
            <Layer2Monitoring detailed />
          </TabsContent>
          <TabsContent value="clients" className="space-y-4">
            <ClientMetrics detailed />
          </TabsContent>
        </Tabs>
      </main>
      <footer className="py-10 flex justify-center items-center text-sm">
        Prototype | Ahmad Zaki Akmal | &copy; 2025
      </footer>
    </div>
  )
}
