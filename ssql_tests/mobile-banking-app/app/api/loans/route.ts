import { type NextRequest, NextResponse } from "next/server";
import { executeQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";

// Function to calculate monthly loan payment (amortization)
function calculateMonthlyPayment(principal: number, annualRate: number, termMonths: number): number {
    if (annualRate <= 0 || termMonths <= 0) {
        return principal / termMonths;
    }
    const monthlyRate = annualRate / 100 / 12;
    const numerator = monthlyRate * Math.pow(1 + monthlyRate, termMonths);
    const denominator = Math.pow(1 + monthlyRate, termMonths) - 1;
    if (denominator === 0) return 0;
    return principal * (numerator / denominator);
}

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const loansQuery = `SELECT * FROM loans WHERE user_id = '${userId}' ORDER BY start_date DESC`;
    const loansFromDb = await executeQuery<any[]>(loansQuery);

    if (!loansFromDb || loansFromDb.length === 0) {
        return NextResponse.json({ success: true, loans: [] });
    }

    const loanIds = loansFromDb.map(l => `'${l.loan_id}'`).join(',');
    const paymentsQuery = `SELECT loan_id, amount, payment_date FROM loan_payments WHERE loan_id IN (${loanIds})`;
    const allPayments = await executeQuery<any[]>(paymentsQuery);

    const paymentsByLoanId = (allPayments || []).reduce((acc, payment) => {
        if (!acc[payment.loan_id]) {
            acc[payment.loan_id] = [];
        }
        acc[payment.loan_id].push(payment);
        return acc;
    }, {} as Record<string, any[]>);

    const loans = loansFromDb.map(loan => {
        const payments = paymentsByLoanId[loan.loan_id] || [];
        const totalPaid = payments.reduce((sum, p) => sum + p.amount, 0);
        const remaining_balance = loan.amount - totalPaid;
        const monthly_payment = calculateMonthlyPayment(loan.amount, loan.interest_rate, loan.term_months);

        let next_payment_date = null;
        if (loan.status === 'Active') {
            const startDate = new Date(loan.start_date);
            // Assuming payments are due on the same day of the month as the start date
            const nextPayment = new Date(startDate.setMonth(startDate.getMonth() + payments.length + 1));
            next_payment_date = nextPayment.toISOString().split('T')[0];
        }

        return {
            ...loan,
            remaining_balance: remaining_balance > 0 ? remaining_balance : 0,
            monthly_payment,
            next_payment_date,
        };
    });

    return NextResponse.json({ success: true, loans });

  } catch (error) {
    console.error("[LOANS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
