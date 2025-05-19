"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Line, LineChart, CartesianGrid, XAxis, YAxis } from "recharts"
import { Clock, ArrowDownUp } from "lucide-react"

const clientData = [
  { time: "12:00", requests: 240, responses: 238, latency: 85 },
  { time: "12:05", requests: 255, responses: 252, latency: 90 },
  { time: "12:10", requests: 270, responses: 268, latency: 95 },
  { time: "12:15", requests: 290, responses: 285, latency: 100 },
  { time: "12:20", requests: 280, responses: 278, latency: 92 },
  { time: "12:25", requests: 275, responses: 272, latency: 88 },
  { time: "12:30", requests: 285, responses: 282, latency: 94 },
]

export function ClientMetrics({ detailed = false }: { detailed?: boolean }) {
  return (
    <>
      <Card className={detailed ? "col-span-full" : ""}>
        <CardHeader>
          <CardTitle>Client Metrics</CardTitle>
          <CardDescription>Request/Response Performance</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <div className="text-sm font-medium">Requests/min</div>
              <div className="text-2xl font-bold">285</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <ArrowDownUp className="mr-1 h-3 w-3" />
                +3.6% from last period
              </div>
            </div>
            <div className="space-y-2">
              <div className="text-sm font-medium">Avg Response Time</div>
              <div className="text-2xl font-bold">94ms</div>
              <div className="flex items-center text-xs text-muted-foreground">
                <Clock className="mr-1 h-3 w-3" />
                +6.8% from last period
              </div>
            </div>
          </div>

          {detailed && (
            <div className="pt-4">
              <h3 className="mb-4 text-sm font-medium">Client Traffic</h3>
              <ChartContainer
                config={{
                  requests: {
                    label: "Requests/min",
                    color: "hsl(var(--chart-1))",
                  },
                  responses: {
                    label: "Responses/min",
                    color: "hsl(var(--chart-2))",
                  },
                  latency: {
                    label: "Latency (ms)",
                    color: "hsl(var(--chart-3))",
                  },
                }}
                className="h-[300px]"
              >
                <LineChart data={clientData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="time" />
                  <YAxis yAxisId="left" orientation="left" />
                  <YAxis yAxisId="right" orientation="right" />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="requests"
                    stroke="var(--color-requests)"
                    activeDot={{ r: 8 }}
                  />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="responses"
                    stroke="var(--color-responses)"
                    strokeDasharray="5 5"
                  />
                  <Line yAxisId="right" type="monotone" dataKey="latency" stroke="var(--color-latency)" />
                </LineChart>
              </ChartContainer>
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <Badge variant="outline" className="bg-green-50 text-green-700">
              98.9% Success Rate
            </Badge>
            <Badge variant="outline" className="bg-amber-50 text-amber-700">
              Increasing Latency
            </Badge>
            <Badge variant="outline" className="bg-blue-50 text-blue-700">
              128 Active Clients
            </Badge>
          </div>
        </CardContent>
      </Card>
    </>
  )
}
