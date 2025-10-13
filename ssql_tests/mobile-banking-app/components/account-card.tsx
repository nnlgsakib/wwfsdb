import { Card, CardContent } from "@/components/ui/card"
import { Wallet, TrendingUp, PiggyBank, BarChart3 } from "lucide-react"
import { cn } from "@/lib/utils"

interface AccountCardProps {
  account: {
    account_id: string
    account_number: string
    account_type: string
    balance: number
    currency: string
    status: string
  }
}

const accountIcons = {
  Checking: Wallet,
  Savings: PiggyBank,
  Investment: TrendingUp,
  Business: BarChart3,
}

const accountColors = {
  Checking: "from-primary to-primary/80",
  Savings: "from-secondary to-secondary/80",
  Investment: "from-accent to-accent/80",
  Business: "from-chart-4 to-chart-4/80",
}

export function AccountCard({ account }: AccountCardProps) {
  const Icon = accountIcons[account.account_type as keyof typeof accountIcons] || Wallet
  const gradient = accountColors[account.account_type as keyof typeof accountColors] || "from-primary to-primary/80"

  return (
    <Card className={cn("overflow-hidden bg-gradient-to-br", gradient, "text-white border-0 shadow-lg")}>
      <CardContent className="p-6">
        <div className="flex items-start justify-between mb-8">
          <div>
            <p className="text-sm opacity-90 mb-1">{account.account_type} Account</p>
            <p className="text-xs opacity-75">{account.account_number}</p>
          </div>
          <div className="w-10 h-10 bg-white/20 rounded-full flex items-center justify-center backdrop-blur-sm">
            <Icon className="w-5 h-5" />
          </div>
        </div>
        <div>
          <p className="text-sm opacity-90 mb-1">Available Balance</p>
          <p className="text-3xl font-bold">
            {account.currency} {(account.balance || 0).toLocaleString("en-US", { minimumFractionDigits: 2 })}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
