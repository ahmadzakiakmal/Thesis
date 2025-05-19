"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ArrowDown, ArrowRight } from "lucide-react"

export function TransactionFlow() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Transaction Flow</CardTitle>
        <CardDescription>Real-time system architecture status</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col items-center justify-center gap-4 md:flex-row">
          {/* Clients Section */}
          <div className="flex flex-col items-center rounded-lg border border-dashed border-yellow-300 bg-yellow-50 p-4 text-center">
            <h3 className="mb-2 font-semibold">Clients</h3>
            <div className="mb-2 text-sm text-muted-foreground">Sending requests (r) and receiving responses (s)</div>
            <div className="grid gap-2">
              <Badge variant="outline" className="bg-blue-50 text-blue-700">
                128 Active Clients
              </Badge>
              <Badge variant="outline" className="bg-green-50 text-green-700">
                285 req/min
              </Badge>
            </div>
          </div>

          {/* Arrows between clients and Layer 2 */}
          <div className="flex flex-col items-center gap-1">
            <ArrowRight className="hidden h-6 w-6 text-muted-foreground md:block" />
            <ArrowDown className="h-6 w-6 text-muted-foreground md:hidden" />
            <div className="text-xs text-muted-foreground">r/s</div>
          </div>

          {/* Layer 2 Section */}
          <div className="flex flex-col items-center rounded-lg border border-dashed border-green-300 bg-green-50 p-4 text-center">
            <h3 className="mb-2 font-semibold">Layer 2</h3>
            <div className="mb-2 text-sm text-muted-foreground">Processing requests with smaller consensus quorum</div>
            <div className="grid gap-2">
              <Badge variant="outline" className="bg-green-50 text-green-700">
                2/2 Nodes Online
              </Badge>
              <Badge variant="outline" className="bg-blue-50 text-blue-700">
                155 Active Sessions
              </Badge>
              <Badge variant="outline" className="bg-blue-50 text-blue-700">
                160 tx/s
              </Badge>
            </div>
          </div>

          {/* Arrows between Layer 2 and Layer 1 */}
          <div className="flex flex-col items-center gap-1">
            <ArrowRight className="hidden h-6 w-6 text-muted-foreground md:block" />
            <ArrowDown className="h-6 w-6 text-muted-foreground md:hidden" />
            <div className="text-xs text-muted-foreground">Commit Tx to L1</div>
          </div>

          {/* Layer 1 Section */}
          <div className="flex flex-col items-center rounded-lg border border-dashed border-blue-300 bg-blue-50 p-4 text-center">
            <h3 className="mb-2 font-semibold">Layer 1</h3>
            <div className="mb-2 text-sm text-muted-foreground">
              Processing committed sessions through BFT consensus
            </div>
            <div className="grid gap-2">
              <Badge variant="outline" className="bg-green-50 text-green-700">
                4/4 Nodes Online
              </Badge>
              <Badge variant="outline" className="bg-green-50 text-green-700">
                BFT Consensus Healthy
              </Badge>
              <Badge variant="outline" className="bg-blue-50 text-blue-700">
                92 tx/s
              </Badge>
            </div>
          </div>
        </div>

        <div className="mt-6 text-sm text-muted-foreground">
          <p className="mb-2">
            <strong>System Status:</strong> The two-layer architecture is functioning properly. Layer 2 is providing
            immediate feedback to clients while Layer 1 ensures tamper-proof transaction records through BFT consensus.
          </p>
          <p>
            <strong>Note:</strong> Layer 2 is experiencing higher than normal transaction volume, which may lead to
            increased latency if the trend continues.
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
