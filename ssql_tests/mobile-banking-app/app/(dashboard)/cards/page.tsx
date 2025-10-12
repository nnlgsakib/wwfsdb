"use client"

import { authFetch } from "@/lib/authFetch";
import { useEffect, useState } from "react"
import { CreditCardComponent } from "@/components/credit-card"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Plus, Lock, Unlock, Trash2 } from "lucide-react"

interface CardData {
  card_id: string
  account_id: string
  card_number: string
  card_type: string
  expiry_date: string
  status: string
  created_at: string
}

export default function CardsPage() {
  const [cards, setCards] = useState<CardData[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchCards = async () => {
      try {
        const response = await authFetch("/api/cards")
        const data = await response.json()
        if (data.success) {
          setCards(data.cards)
        }
      } catch (error) {
        console.error("[v0] Error fetching cards:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchCards()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="p-4 md:p-8 space-y-8 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Cards</h1>
          <p className="text-muted-foreground mt-1">Manage your debit and credit cards</p>
        </div>
        <Button className="gap-2">
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">Request Card</span>
        </Button>
      </div>

      {/* Cards Display */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {cards.map((card) => (
          <CreditCardComponent key={card.card_id} card={card} />
        ))}
      </div>

      {/* Card Management */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {cards.map((card) => (
          <Card key={card.card_id}>
            <CardHeader>
              <CardTitle className="text-lg">
                {card.card_type} Card - {card.card_number.slice(-4)}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Status</span>
                  <span className="font-medium text-success">{card.status}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Expires</span>
                  <span className="font-medium">{card.expiry_date}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Type</span>
                  <span className="font-medium">{card.card_type}</span>
                </div>
              </div>

              <div className="flex gap-2 pt-2">
                <Button variant="outline" size="sm" className="flex-1 gap-2 bg-transparent">
                  <Lock className="w-4 h-4" />
                  Lock
                </Button>
                <Button variant="outline" size="sm" className="flex-1 gap-2 bg-transparent">
                  <Unlock className="w-4 h-4" />
                  Unlock
                </Button>
              </div>

              <Button variant="destructive" size="sm" className="w-full gap-2">
                <Trash2 className="w-4 h-4" />
                Report Lost
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
