"use client"

import { authFetch } from "@/lib/authFetch";
import type React from "react"

import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Plus, Trash2, Building2, User } from "lucide-react"

interface Beneficiary {
  beneficiary_id: string
  nickname: string
  account_number: string
  bank_name: string
  created_at: string
}

export default function BeneficiariesPage() {
  const [beneficiaries, setBeneficiaries] = useState<Beneficiary[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [formData, setFormData] = useState({
    nickname: "",
    account_number: "",
    bank_name: "",
  })

  useEffect(() => {
    fetchBeneficiaries()
  }, [])

  const fetchBeneficiaries = async () => {
    try {
      const response = await authFetch("/api/beneficiaries")
      const data = await response.json()
      if (data.success) {
        setBeneficiaries(data.beneficiaries)
      }
    } catch (error) {
      console.error("[v0] Error fetching beneficiaries:", error)
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    try {
      const response = await authFetch("/api/beneficiaries", {
        method: "POST",
        body: JSON.stringify(formData),
      })

      const data = await response.json()

      if (data.success) {
        setBeneficiaries([...beneficiaries, data.beneficiary])
        setFormData({ nickname: "", account_number: "", bank_name: "" })
        setDialogOpen(false)
      }
    } catch (error) {
      console.error("[v0] Error adding beneficiary:", error)
    }
  }

  const handleDelete = async (beneficiary_id: string) => {
    try {
      const response = await authFetch("/api/beneficiaries", {
        method: "DELETE",
        body: JSON.stringify({ beneficiary_id }),
      })

      const data = await response.json()

      if (data.success) {
        setBeneficiaries(beneficiaries.filter((b) => b.beneficiary_id !== beneficiary_id))
      }
    } catch (error) {
      console.error("[v0] Error deleting beneficiary:", error)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="p-4 md:p-8 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Beneficiaries</h1>
          <p className="text-muted-foreground mt-1">Manage your saved beneficiaries for quick transfers</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger asChild>
            <Button className="gap-2">
              <Plus className="w-4 h-4" />
              <span className="hidden sm:inline">Add Beneficiary</span>
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add New Beneficiary</DialogTitle>
              <DialogDescription>Add a new beneficiary for quick money transfers</DialogDescription>
            </DialogHeader>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="nickname">Nickname</Label>
                <Input
                  id="nickname"
                  placeholder="e.g., Mom, John Smith"
                  value={formData.nickname}
                  onChange={(e) => setFormData({ ...formData, nickname: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account_number">Account Number</Label>
                <Input
                  id="account_number"
                  placeholder="Enter account number"
                  value={formData.account_number}
                  onChange={(e) => setFormData({ ...formData, account_number: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="bank_name">Bank Name</Label>
                <Input
                  id="bank_name"
                  placeholder="e.g., SecureBank"
                  value={formData.bank_name}
                  onChange={(e) => setFormData({ ...formData, bank_name: e.target.value })}
                  required
                />
              </div>
              <Button type="submit" className="w-full">
                Add Beneficiary
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {/* Beneficiaries Grid */}
      {beneficiaries.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16">
            <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mb-4">
              <User className="w-8 h-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-2">No beneficiaries yet</h3>
            <p className="text-muted-foreground text-center mb-4">
              Add beneficiaries to make transfers faster and easier
            </p>
            <Button onClick={() => setDialogOpen(true)} className="gap-2">
              <Plus className="w-4 h-4" />
              Add Your First Beneficiary
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {beneficiaries.map((beneficiary) => (
            <Card key={beneficiary.beneficiary_id} className="hover:shadow-lg transition-shadow">
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-12 h-12 bg-primary/10 rounded-full flex items-center justify-center">
                      <User className="w-6 h-6 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-lg">{beneficiary.nickname}</CardTitle>
                      <p className="text-sm text-muted-foreground">{beneficiary.account_number}</p>
                    </div>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Building2 className="w-4 h-4" />
                  <span>{beneficiary.bank_name}</span>
                </div>
                <div className="text-xs text-muted-foreground">
                  Added on{" "}
                  {new Date(beneficiary.created_at).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}
                </div>
                <div className="flex gap-2 pt-2">
                  <Button variant="outline" size="sm" className="flex-1 bg-transparent">
                    Transfer
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => handleDelete(beneficiary.beneficiary_id)}
                    className="gap-2"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
