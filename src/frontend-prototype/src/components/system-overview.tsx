"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { CheckCircle2, AlertTriangle, Activity, Server, Users, ArrowDownUp, LineChart } from "lucide-react"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts"

const systemMetrics = [
  { name: "00:00", layer1: 65, layer2: 120 },
  { name: "01:00", layer1: 68, layer2: 125 },
  { name: "02:00", layer1: 75, layer2: 130 },
  { name: "03:00", layer1: 70, layer2: 118 },
  { name: "04:00", layer1: 72, layer2: 122 },
  { name: "05:00", layer1: 78, layer2: 135 },
  { name: "06:00", layer1: 82, layer2: 140 },
  { name: "07:00", layer1: 80, layer2: 138 },
  { name: "08:00", layer1: 85, layer2: 145 },
  { name: "09:00", layer1: 88, layer2: 150 },
  { name: "10:00", layer1: 90, layer2: 155 },
  { name: "11:00", layer1: 92, layer2: 160 },
]

export function SystemOverview() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Layer 1 Nodes</CardTitle>
          <Server className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">4/4</div>
          <p className="text-xs text-muted-foreground">All nodes operational</p>
          <div className="mt-2 flex items-center space-x-1 text-sm text-green-600">
            <CheckCircle2 className="h-4 w-4" />
            <span>BFT Consensus Healthy</span>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Layer 2 Nodes</CardTitle>
          <Server className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">2/2</div>
          <p className="text-xs text-muted-foreground">All nodes operational</p>
          <div className="mt-2 flex items-center space-x-1 text-sm text-green-600">
            <CheckCircle2 className="h-4 w-4" />
            <span>Quorum Healthy</span>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Active Clients</CardTitle>
          <Users className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">128</div>
          <p className="text-xs text-muted-foreground">+22% from last hour</p>
          <div className="mt-2 flex items-center space-x-1 text-sm text-green-600">
            <Activity className="h-4 w-4" />
            <span>Normal Load</span>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Tx Throughput</CardTitle>
          <ArrowDownUp className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">1,245 tx/s</div>
          <p className="text-xs text-muted-foreground">Layer 2: 160 tx/s, Layer 1: 92 tx/s</p>
          <div className="mt-2 flex items-center space-x-1 text-sm text-amber-600">
            <AlertTriangle className="h-4 w-4" />
            <span>High Load</span>
          </div>
        </CardContent>
      </Card>
      <Card className="col-span-full">
        <CardHeader>
          <CardTitle>System Performance</CardTitle>
          <CardDescription>Transaction throughput across layers over the last 12 hours</CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer
            config={{
              layer1: {
                label: "Layer 1 (tx/s)",
                color: "#e76e50",
              },
              layer2: {
                label: "Layer 2 (tx/s)",
                color: "#56a0be",
              },
            }}
            className="aspect-[4/1]"
          >
            <AreaChart data={systemMetrics} margin={{ top: 10, right: 30, left: 0, bottom: 0 }}>
              <defs>
                <LineChart id="colorLayer1" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#e76e50" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="#e76e50" stopOpacity={0} />
                </LineChart>
                <LineChart id="colorLayer2" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#56a0be" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="#56a0be" stopOpacity={0} />
                </LineChart>
              </defs>
              <XAxis dataKey="name" />
              <YAxis />
              <CartesianGrid strokeDasharray="3 3" />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Area
                type="monotone"
                dataKey="layer1"
                stroke="#e76e50"
                fillOpacity={1}
                fill="url(#colorLayer1)"
              />
              <Area
                type="monotone"
                dataKey="layer2"
                stroke="#56a0be"
                fillOpacity={1}
                fill="url(#colorLayer2)"
              />
            </AreaChart>
          </ChartContainer>
        </CardContent>
      </Card>
      <Alert className="col-span-full">
        <AlertTriangle className="h-4 w-4" />
        <AlertTitle>High Transaction Load</AlertTitle>
        <AlertDescription>
          Layer 2 is experiencing higher than normal transaction volume. Consider scaling up resources if this trend
          continues.
        </AlertDescription>
      </Alert>
    </div>
  )
}
