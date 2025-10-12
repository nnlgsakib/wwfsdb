import { type NextRequest, NextResponse } from "next/server";
import { executeQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";

export async function GET(
  request: NextRequest,
  { params }: { params: { loanId: string } }
) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const { loanId } = params;

    // Verify that the loan belongs to the user
    const loanQuery = `SELECT user_id FROM loans WHERE loan_id = '${loanId}'`;
    const loans = await executeQuery<any[]>(loanQuery);

    if (!loans || loans.length === 0 || loans[0].user_id !== userId) {
        return NextResponse.json(
            { success: false, message: "Loan not found or access denied" },
            { status: 404 }
        );
    }

    const paymentsQuery = `SELECT * FROM loan_payments WHERE loan_id = '${loanId}' ORDER BY payment_date DESC`;
    const payments = await executeQuery<any[]>(paymentsQuery);

    return NextResponse.json({ success: true, payments: payments || [] });

  } catch (error) {
    console.error("[LOAN_PAYMENTS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
