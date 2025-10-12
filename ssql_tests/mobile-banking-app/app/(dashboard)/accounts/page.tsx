"use client"

import { authFetch } from "@/lib/authFetch";
import { useEffect, useState } from "react"
import { AccountCard } from "@/components/account-card"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Plus, TrendingUp, TrendingDown } from "lucide-react"

interface Account {
  account_id: string
  account_number: string
  account_type: string
  balance: number
  currency: string
  status: string
  created_at: string
}

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchAccounts = async () => {
      try {
        const response = await authFetch("/api/accounts")
        const data = await response.json()
        if (data.success) {
          setAccounts(data.accounts)
        }
      } catch (error) {
        console.error("[v0] Error fetching accounts:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchAccounts()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  const totalBalance = accounts.reduce((sum, acc) => sum + acc.balance, 0)

  return (
    <div className="p-4 md:p-8 space-y-8 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Accounts</h1>
          <p className="text-muted-foreground mt-1">Manage all your bank accounts</p>
        </div>
        <Button className="gap-2">
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">New Account</span>
        </Button>
      </div>

      {/* Total Balance Card */}
      <Card className="bg-gradient-to-br from-primary via-primary to-primary/80 text-white border-0 shadow-xl">
        <CardContent className="p-8">
          <p className="text-sm opacity-90 mb-2">Total Balance Across All Accounts</p>
          <p className="text-4xl md:text-5xl font-bold mb-6">
            USD {totalBalance.toLocaleString("en-US", { minimumFractionDigits: 2 })}
          </p>
          <div className="flex gap-4">
            <div className="flex items-center gap-2 bg-white/20 backdrop-blur-sm px-4 py-2 rounded-lg">
              <TrendingUp className="w-4 h-4" />
              <span className="text-sm">+12.5% this month</span>
            </div>
            <div className="flex items-center gap-2 bg-white/20 backdrop-blur-sm px-4 py-2 rounded-lg">
              <TrendingDown className="w-4 h-4" />
              <span className="text-sm">-3.2% expenses</span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Accounts Grid */}
      <div>
        <h2 className="text-xl font-bold mb-4">Your Accounts</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {accounts.map((account) => (
            <AccountCard key={account.account_id} account={account} />
          ))}
        </div>
      </div>

      {/* Account Details */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {accounts.map((account) => (
          <Card key={account.account_id}>
            <CardHeader>
              <CardTitle className="text-lg">{account.account_type} Account Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex justify-between py-2 border-b">
                <span className="text-muted-foreground">Account Number</span>
                <span className="font-medium">{account.account_number}</span>
              </div>
              <div className="flex justify-between py-2 border-b">
                <span className="text-muted-foreground">Account Type</span>
                <span className="font-medium">{account.account_type}</span>
              </div>
              <div className="flex justify-between py-2 border-b">
                <span className="text-muted-foreground">Status</span>
                <span className="font-medium text-success">{account.status}</span>
              </div>
              <div className="flex justify-between py-2 border-b">
                <span className="text-muted-foreground">Currency</span>
                <span className="font-medium">{account.currency}</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground">Opened On</span>
                <span className="font-medium">
                  {new Date(account.created_at).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}
                </span>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
