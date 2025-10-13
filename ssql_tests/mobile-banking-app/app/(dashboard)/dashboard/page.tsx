"use client"

import { authFetch } from "@/lib/authFetch";
import { useEffect, useState } from "react"
import { AccountCard } from "@/components/account-card"
import { StatCard } from "@/components/stat-card"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { ArrowUpRight, ArrowDownRight, DollarSign, CreditCard, TrendingUp, Send } from "lucide-react"
import Link from "next/link"

interface DashboardData {
  stats: {
    total_balance: number
    monthly_income: number
    monthly_expenses: number
    active_cards: number
    recent_transactions: Array<{
      transaction_id: string
      amount: number
      currency: string
      transaction_type: string
      description: string
      created_at: string
    }>
  }
  accounts: Array<{
    account_id: string
    account_number: string
    account_type: string
    balance: number
    currency: string
    status: string
  }>
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [statsRes, accountsRes] = await Promise.all([
          authFetch("/api/dashboard/stats"), 
          authFetch("/api/accounts")
        ]);

        const statsData = await statsRes.json();
        const accountsData = await accountsRes.json();

        if (statsData.success && accountsData.success) {
            setData({
              stats: statsData.stats,
              accounts: accountsData.accounts,
            });
        } else {
            console.error("Failed to fetch dashboard data:", statsData.message, accountsData.message);
        }

      } catch (error) {
        console.error("[v0] Error fetching dashboard data:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  if (!data) return null

  return (
    <div className="p-4 md:p-8 space-y-8 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Dashboard</h1>
          <p className="text-muted-foreground mt-1">Welcome back! Here's your financial overview</p>
        </div>
        <Link href="/transfer">
          <Button className="gap-2">
            <Send className="w-4 h-4" />
            <span className="hidden sm:inline">Transfer Money</span>
          </Button>
        </Link>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Total Balance"
          value={`${(data.stats.total_balance || 0).toLocaleString()}`}
          icon={DollarSign}
          trend={{ value: "12.5%", positive: true }}
        />
        <StatCard
          title="Monthly Income"
          value={`${(data.stats.monthly_income || 0).toLocaleString()}`}
          icon={TrendingUp}
          trend={{ value: "8.2%", positive: true }}
        />
        <StatCard
          title="Monthly Expenses"
          value={`${(data.stats.monthly_expenses || 0).toLocaleString()}`}
          icon={ArrowDownRight}
          trend={{ value: "3.1%", positive: false }}
        />
        <StatCard title="Active Cards" value={data.stats.active_cards.toString()} icon={CreditCard} />
      </div>

      {/* Accounts */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold">Your Accounts</h2>
          <Link href="/accounts">
            <Button variant="ghost" size="sm">
              View All
            </Button>
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {data.accounts.map((account) => (
            <AccountCard key={account.account_id} account={account} />
          ))}
        </div>
      </div>

      {/* Recent Transactions */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Recent Transactions</CardTitle>
          <Link href="/transactions">
            <Button variant="ghost" size="sm">
              View All
            </Button>
          </Link>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {data.stats.recent_transactions.map((transaction) => (
              <div
                key={transaction.transaction_id}
                className="flex items-center justify-between py-3 border-b last:border-0"
              >
                <div className="flex items-center gap-3">
                  <div
                    className={`w-10 h-10 rounded-full flex items-center justify-center ${
                      transaction.amount > 0 ? "bg-success/10" : "bg-muted"
                    }`}
                  >
                    {transaction.amount > 0 ? (
                      <ArrowDownRight className="w-5 h-5 text-success" />
                    ) : (
                      <ArrowUpRight className="w-5 h-5 text-muted-foreground" />
                    )}
                  </div>
                  <div>
                    <p className="font-medium">{transaction.description}</p>
                    <p className="text-sm text-muted-foreground">
                      {new Date(transaction.created_at).toLocaleDateString("en-US", {
                        month: "short",
                        day: "numeric",
                        year: "numeric",
                      })}
                    </p>
                  </div>
                </div>
                <p className={`font-semibold ${transaction.amount > 0 ? "text-success" : "text-foreground"}`}>
                  {transaction.amount > 0 ? "+" : ""}
                  {transaction.currency}{" "}
                  {Math.abs(transaction.amount || 0).toLocaleString("en-US", { minimumFractionDigits: 2 })}
                </p>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
