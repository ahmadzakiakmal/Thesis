"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"
import { CheckCircle, Clock, Server } from "lucide-react"

const nodeData = [
  { name: "Node 1", transactions: 24, latency: 120, cpu: 65, memory: 72 },
  { name: "Node 2", transactions: 22, latency: 115, cpu: 58, memory: 68 },
  { name: "Node 3", transactions: 25, latency: 125, cpu: 70, memory: 75 },
  { name: "Node 4", transactions: 21, latency: 118, cpu: 62, memory: 70 },
]

const consensusData = [
  { name: "Round 1", duration: 850 },
  { name: "Round 2", duration: 920 },
  { name: "Round 3", duration: 880 },
  { name: "Round 4", duration: 900 },
  { name: "Round 5", duration: 950 },
]

export function Layer1Monitoring({ detailed = false }: { detailed?: boolean }) {
  return (
    <>
      <Card className={detailed ? "col-span-full" : ""}>
        <CardHeader>
          <CardTitle>Layer 1 Monitoring</CardTitle>
          <CardDescription>BFT Consensus Network Status</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <div className="text-sm font-medium">Consensus Rounds</div>
              <div className="text-2xl font-bold">5,432</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <Clock className="mr-1 h-3 w-3" />
                Last round: 2s ago
              </div>
            </div>
            <div className="space-y-2">
              <div className="text-sm font-medium">Committed Transactions</div>
              <div className="text-2xl font-bold">92,145</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <CheckCircle className="mr-1 h-3 w-3" />
                100% success rate
              </div>
            </div>
          </div>

          {detailed && (
            <>
              <div className="pt-4">
                <h3 className="mb-4 text-sm font-medium">Node Status</h3>
                <div className="space-y-4">
                  {nodeData.map((node) => (
                    <div key={node.name} className="grid grid-cols-5 gap-4">
                      <div className="flex items-center gap-2">
                        <Server className="h-4 w-4 text-muted-foreground" />
                        <span className="font-medium">{node.name}</span>
                      </div>
                      <div>
                        <div className="text-xs text-muted-foreground">CPU</div>
                        <Progress value={node.cpu} className="h-2" />
                        <div className="text-xs text-right">{node.cpu}%</div>
                      </div>
                      <div>
                        <div className="text-xs text-muted-foreground">Memory</div>
                        <Progress value={node.memory} className="h-2" />
                        <div className="text-xs text-right">{node.memory}%</div>
                      </div>
                      <div>
                        <div className="text-xs text-muted-foreground">Tx/s</div>
                        <div className="text-sm font-medium">{node.transactions}</div>
                      </div>
                      <div>
                        <div className="text-xs text-muted-foreground">Latency</div>
                        <div className="text-sm font-medium">{node.latency}ms</div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4">
                <h3 className="mb-4 text-sm font-medium">BFT Consensus Performance</h3>
                <ChartContainer
                  config={{
                    duration: {
                      label: "Duration (ms)",
                      color: "hsl(var(--chart-1))",
                    },
                  }}
                  className="h-[200px]"
                >
                  <BarChart data={consensusData}>
                    <CartesianGrid strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="name" />
                    <YAxis />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Bar dataKey="duration" fill="var(--color-duration)" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ChartContainer>
              </div>
            </>
          )}

          <div className="flex flex-wrap gap-2">
            <Badge variant="outline" className="bg-green-50 text-green-700">
              BFT Healthy
            </Badge>
            <Badge variant="outline" className="bg-green-50 text-green-700">
              All Nodes Online
            </Badge>
            <Badge variant="outline" className="bg-green-50 text-green-700">
              Tamper-Proof
            </Badge>
            <Badge variant="outline" className="bg-blue-50 text-blue-700">
              92 tx/s
            </Badge>
          </div>
        </CardContent>
      </Card>
    </>
  )
}
