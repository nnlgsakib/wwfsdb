import { Card } from "@/components/ui/card"
import { CreditCardIcon, Lock } from "lucide-react"
import { cn } from "@/lib/utils"

interface CreditCardProps {
  card: {
    card_id: string
    card_number: string
    card_type: string
    expiry_date: string
    status: string
  }
  className?: string
}

const cardGradients = {
  Debit: "from-primary via-primary/90 to-primary/80",
  Credit: "from-secondary via-secondary/90 to-secondary/80",
}

export function CreditCardComponent({ card, className }: CreditCardProps) {
  const gradient = cardGradients[card.card_type as keyof typeof cardGradients] || "from-primary to-primary/80"

  return (
    <Card
      className={cn(
        "relative overflow-hidden bg-gradient-to-br border-0 shadow-xl text-white p-6 aspect-[1.586/1]",
        gradient,
        className,
      )}
    >
      {/* Background pattern */}
      <div className="absolute inset-0 opacity-10">
        <div className="absolute top-0 right-0 w-64 h-64 bg-white rounded-full -translate-y-1/2 translate-x-1/2" />
        <div className="absolute bottom-0 left-0 w-48 h-48 bg-white rounded-full translate-y-1/2 -translate-x-1/2" />
      </div>

      <div className="relative h-full flex flex-col justify-between">
        {/* Card Header */}
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xs opacity-90 mb-1">SecureBank</p>
            <p className="text-xs opacity-75">{card.card_type} Card</p>
          </div>
          <div className="w-10 h-10 bg-white/20 rounded-lg flex items-center justify-center backdrop-blur-sm">
            <CreditCardIcon className="w-5 h-5" />
          </div>
        </div>

        {/* Card Number */}
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            <Lock className="w-3 h-3 opacity-75" />
            <p className="text-lg font-mono tracking-wider">{card.card_number}</p>
          </div>

          {/* Card Footer */}
          <div className="flex items-end justify-between">
            <div>
              <p className="text-xs opacity-75 mb-1">Valid Thru</p>
              <p className="text-sm font-medium">{card.expiry_date}</p>
            </div>
            <div className="flex gap-1">
              <div className="w-8 h-8 bg-white/30 rounded-full backdrop-blur-sm" />
              <div className="w-8 h-8 bg-white/50 rounded-full backdrop-blur-sm -ml-3" />
            </div>
          </div>
        </div>
      </div>
    </Card>
  )
}
