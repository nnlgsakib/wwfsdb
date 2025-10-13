"use client"

import { authFetch } from "@/lib/authFetch";
import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { Plus, Calendar, DollarSign, TrendingDown, CheckCircle2 } from "lucide-react"
import Link from "next/link"

interface Loan {
  loan_id: string
  loan_type: string
  amount: number
  interest_rate: number
  term_months: number
  status: string
  start_date: string
  end_date: string
  remaining_balance: number
  monthly_payment: number
  next_payment_date: string | null
}

export default function LoansPage() {
  const [loans, setLoans] = useState<Loan[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchLoans = async () => {
      try {
        const response = await authFetch("/api/loans")
        const data = await response.json()
        if (data.success) {
          setLoans(data.loans)
        }
      } catch (error) {
        console.error("[v0] Error fetching loans:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchLoans()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  const activeLoans = loans.filter((loan) => loan.status === "Active")
  const totalBorrowed = activeLoans.reduce((sum, loan) => sum + (loan.amount || 0), 0)
  const totalRemaining = activeLoans.reduce((sum, loan) => sum + (loan.remaining_balance || 0), 0)

  return (
    <div className="p-4 md:p-8 space-y-8 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Loans</h1>
          <p className="text-muted-foreground mt-1">Manage your loans and payment schedules</p>
        </div>
        <Button className="gap-2">
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">Apply for Loan</span>
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardContent className="p-6">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm text-muted-foreground mb-2">Total Borrowed</p>
                <p className="text-2xl font-bold">${(totalBorrowed || 0).toLocaleString()}</p>
              </div>
              <div className="w-12 h-12 bg-primary/10 rounded-xl flex items-center justify-center">
                <DollarSign className="w-6 h-6 text-primary" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-6">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm text-muted-foreground mb-2">Remaining Balance</p>
                <p className="text-2xl font-bold">${(totalRemaining || 0).toLocaleString()}</p>
              </div>
              <div className="w-12 h-12 bg-secondary/10 rounded-xl flex items-center justify-center">
                <TrendingDown className="w-6 h-6 text-secondary" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-6">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm text-muted-foreground mb-2">Active Loans</p>
                <p className="text-2xl font-bold">{activeLoans.length}</p>
              </div>
              <div className="w-12 h-12 bg-accent/10 rounded-xl flex items-center justify-center">
                <CheckCircle2 className="w-6 h-6 text-accent" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Loans List */}
      <div className="space-y-4">
        {loans.map((loan) => {
          const progress = ((loan.amount - loan.remaining_balance) / loan.amount) * 100

          return (
            <Card key={loan.loan_id} className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <div className="flex items-start justify-between">
                  <div>
                    <CardTitle className="text-xl">{loan.loan_type} Loan</CardTitle>
                    <p className="text-sm text-muted-foreground mt-1">
                      {loan.status === "Active" ? `${loan.term_months} months term` : "Completed"}
                    </p>
                  </div>
                  <span
                    className={`px-3 py-1 rounded-full text-xs font-medium ${
                      loan.status === "Active"
                        ? "bg-success/10 text-success"
                        : loan.status === "Paid Off"
                          ? "bg-primary/10 text-primary"
                          : "bg-muted text-muted-foreground"
                    }`}
                  >
                    {loan.status}
                  </span>
                </div>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Progress Bar */}
                {loan.status === "Active" && (
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Repayment Progress</span>
                      <span className="font-medium">{progress.toFixed(1)}%</span>
                    </div>
                    <Progress value={progress} className="h-2" />
                  </div>
                )}

                {/* Loan Details Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground mb-1">Loan Amount</p>
                    <p className="font-semibold">${(loan.amount || 0).toLocaleString()}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground mb-1">Remaining</p>
                    <p className="font-semibold">${(loan.remaining_balance || 0).toLocaleString()}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground mb-1">Interest Rate</p>
                    <p className="font-semibold">{loan.interest_rate}%</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground mb-1">Monthly Payment</p>
                    <p className="font-semibold">
                      {loan.monthly_payment > 0 ? `${(loan.monthly_payment || 0).toLocaleString()}` : "N/A"}
                    </p>
                  </div>
                </div>

                {/* Next Payment */}
                {loan.status === "Active" && loan.next_payment_date && (
                  <div className="flex items-center justify-between p-4 bg-muted rounded-lg">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 bg-primary/10 rounded-full flex items-center justify-center">
                        <Calendar className="w-5 h-5 text-primary" />
                      </div>
                      <div>
                        <p className="text-sm text-muted-foreground">Next Payment Due</p>
                        <p className="font-semibold">
                          {new Date(loan.next_payment_date).toLocaleDateString("en-US", {
                            month: "long",
                            day: "numeric",
                            year: "numeric",
                          })}
                        </p>
                      </div>
                    </div>
                    <Link href={`/loans/${loan.loan_id}`}>
                      <Button size="sm">Make Payment</Button>
                    </Link>
                  </div>
                )}

                {/* Action Buttons */}
                <div className="flex gap-2 pt-2">
                  <Link href={`/loans/${loan.loan_id}`} className="flex-1">
                    <Button variant="outline" className="w-full bg-transparent">
                      View Details
                    </Button>
                  </Link>
                  {loan.status === "Active" && (
                    <Button variant="outline" className="flex-1 bg-transparent">
                      Payment History
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </div>
  )
}
