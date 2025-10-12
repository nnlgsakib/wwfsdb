import { type NextRequest, NextResponse } from "next/server";
import { executeQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const accountsQuery = `SELECT account_id FROM accounts WHERE user_id = '${userId}'`;
    const userAccounts = await executeQuery<any[]>(accountsQuery);

    if (!userAccounts || userAccounts.length === 0) {
        return NextResponse.json({ success: true, transactions: [] });
    }

    const accountIds = userAccounts.map(acc => `'${acc.account_id}'`).join(', ');
    const userAccountIdsQuery = `(${accountIds})`;

    const query = `
      SELECT * FROM transactions 
      WHERE from_account_id IN ${userAccountIdsQuery} 
         OR to_account_id IN ${userAccountIdsQuery}
      ORDER BY created_at DESC`;
      
    const transactions = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, transactions: transactions || [] });

  } catch (error) {
    console.error("[TRANSACTIONS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
