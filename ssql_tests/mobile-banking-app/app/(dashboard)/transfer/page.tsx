"use client"

import { authFetch } from "@/lib/authFetch";
import type React from "react"

import { useEffect, useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { Send, Users, CheckCircle2 } from "lucide-react"
import { useRouter } from "next/navigation"

interface Account {
  account_id: string
  account_number: string
  account_type: string
  balance: number
  currency: string
}

interface Beneficiary {
  beneficiary_id: string
  nickname: string
  account_number: string
  bank_name: string
}

export default function TransferPage() {
  const router = useRouter()
  const [accounts, setAccounts] = useState<Account[]>([])
  const [beneficiaries, setBeneficiaries] = useState<Beneficiary[]>([])
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState("")

  const [formData, setFormData] = useState({
    from_account_id: "",
    to_account_number: "",
    amount: "",
    description: "",
  })

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [accountsRes, beneficiariesRes] = await Promise.all([authFetch("/api/accounts"), authFetch("/api/beneficiaries")])

        const accountsData = await accountsRes.json()
        const beneficiariesData = await beneficiariesRes.json()

        if (accountsData.success) setAccounts(accountsData.accounts)
        if (beneficiariesData.success) setBeneficiaries(beneficiariesData.beneficiaries)
      } catch (error) {
        console.error("[v0] Error fetching data:", error)
      }
    }

    fetchData()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError("")

    try {
      const response = await authFetch("/api/transfer", {
        method: "POST",
        body: JSON.stringify(formData),
      })

      const data = await response.json()

      if (data.success) {
        setSuccess(true)
        setTimeout(() => {
          router.push("/transactions")
        }, 2000)
      } else {
        setError(data.message || "Transfer failed")
      }
    } catch (err) {
      setError("An error occurred. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  const selectBeneficiary = (beneficiary: Beneficiary) => {
    setFormData({ ...formData, to_account_number: beneficiary.account_number })
  }

  if (success) {
    return (
      <div className="flex items-center justify-center min-h-screen p-4">
        <Card className="w-full max-w-md text-center">
          <CardContent className="pt-12 pb-12">
            <div className="w-20 h-20 bg-success/10 rounded-full flex items-center justify-center mx-auto mb-6">
              <CheckCircle2 className="w-10 h-10 text-success" />
            </div>
            <h2 className="text-2xl font-bold mb-2">Transfer Successful!</h2>
            <p className="text-muted-foreground mb-6">Your money has been transferred successfully</p>
            <Button onClick={() => router.push("/transactions")} className="w-full">
              View Transactions
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="p-4 md:p-8 space-y-6 max-w-4xl mx-auto">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-balance">Transfer Money</h1>
        <p className="text-muted-foreground mt-1">Send money to your beneficiaries or any account</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Transfer Form */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Send className="w-5 h-5" />
              Transfer Details
            </CardTitle>
            <CardDescription>Enter the transfer information below</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="from_account">From Account</Label>
                <Select
                  value={formData.from_account_id}
                  onValueChange={(value) => setFormData({ ...formData, from_account_id: value })}
                >
                  <SelectTrigger id="from_account">
                    <SelectValue placeholder="Select account" />
                  </SelectTrigger>
                  <SelectContent>
                    {accounts.map((account) => (
                      <SelectItem key={account.account_id} value={account.account_id}>
                        {account.account_type} - {account.account_number} (${account.balance.toLocaleString()})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="to_account">To Account Number</Label>
                <Input
                  id="to_account"
                  placeholder="Enter account number"
                  value={formData.to_account_number}
                  onChange={(e) => setFormData({ ...formData, to_account_number: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="amount">Amount (USD)</Label>
                <Input
                  id="amount"
                  type="number"
                  step="0.01"
                  placeholder="0.00"
                  value={formData.amount}
                  onChange={(e) => setFormData({ ...formData, amount: e.target.value })}
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="description">Description (Optional)</Label>
                <Textarea
                  id="description"
                  placeholder="What's this transfer for?"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  rows={3}
                />
              </div>

              {error && <div className="text-sm text-destructive bg-destructive/10 p-3 rounded-lg">{error}</div>}

              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? "Processing..." : "Transfer Money"}
              </Button>
            </form>
          </CardContent>
        </Card>

        {/* Beneficiaries */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Users className="w-5 h-5" />
              Beneficiaries
            </CardTitle>
            <CardDescription>Quick select</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {beneficiaries.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">No beneficiaries added yet</p>
            ) : (
              beneficiaries.map((beneficiary) => (
                <button
                  key={beneficiary.beneficiary_id}
                  type="button"
                  onClick={() => selectBeneficiary(beneficiary)}
                  className="w-full p-3 rounded-lg border hover:bg-muted transition-colors text-left"
                >
                  <p className="font-medium">{beneficiary.nickname}</p>
                  <p className="text-sm text-muted-foreground">{beneficiary.account_number}</p>
                  <p className="text-xs text-muted-foreground mt-1">{beneficiary.bank_name}</p>
                </button>
              ))
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
