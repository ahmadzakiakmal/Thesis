"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Line, LineChart, CartesianGrid, XAxis, YAxis } from "recharts"
import { ArrowDownToLine, Clock, Server } from "lucide-react"

const nodeData = [
  { name: "Node 1", transactions: 82, latency: 45, cpu: 78, memory: 85 },
  { name: "Node 2", transactions: 78, latency: 42, cpu: 75, memory: 80 },
]

const sessionData = [
  { time: "12:00", sessions: 120, latency: 45 },
  { time: "12:05", sessions: 132, latency: 48 },
  { time: "12:10", sessions: 145, latency: 52 },
  { time: "12:15", sessions: 160, latency: 55 },
  { time: "12:20", sessions: 152, latency: 50 },
  { time: "12:25", sessions: 148, latency: 47 },
  { time: "12:30", sessions: 155, latency: 49 },
]

export function Layer2Monitoring({ detailed = false }: { detailed?: boolean }) {
  return (
    <>
      <Card className={detailed ? "col-span-full" : ""}>
        <CardHeader>
          <CardTitle>Layer 2 Monitoring</CardTitle>
          <CardDescription>Fast Feedback Session Processing</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <div className="text-sm font-medium">Active Sessions</div>
              <div className="text-2xl font-bold">155</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <Clock className="mr-1 h-3 w-3" />
                Avg duration: 2.5s
              </div>
            </div>
            <div className="space-y-2">
              <div className="text-sm font-medium">Commits to Layer 1</div>
              <div className="text-2xl font-bold">1,245</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <ArrowDownToLine className="mr-1 h-3 w-3" />
                Last commit: 5s ago
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
                <h3 className="mb-4 text-sm font-medium">Session Performance</h3>
                <ChartContainer
                  config={{
                    sessions: {
                      label: "Active Sessions",
                      color: "hsl(var(--chart-1))",
                    },
                    latency: {
                      label: "Latency (ms)",
                      color: "hsl(var(--chart-2))",
                    },
                  }}
                  className="h-[200px]"
                >
                  <LineChart data={sessionData}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="time" />
                    <YAxis yAxisId="left" orientation="left" />
                    <YAxis yAxisId="right" orientation="right" />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Line
                      yAxisId="left"
                      type="monotone"
                      dataKey="sessions"
                      stroke="var(--color-sessions)"
                      activeDot={{ r: 8 }}
                    />
                    <Line yAxisId="right" type="monotone" dataKey="latency" stroke="var(--color-latency)" />
                  </LineChart>
                </ChartContainer>
              </div>
            </>
          )}

          <div className="flex flex-wrap gap-2">
            <Badge variant="outline" className="bg-green-50 text-green-700">
              Quorum Healthy
            </Badge>
            <Badge variant="outline" className="bg-green-50 text-green-700">
              All Nodes Online
            </Badge>
            <Badge variant="outline" className="bg-amber-50 text-amber-700">
              High Load
            </Badge>
            <Badge variant="outline" className="bg-blue-50 text-blue-700">
              160 tx/s
            </Badge>
          </div>
        </CardContent>
      </Card>
    </>
  )
}
