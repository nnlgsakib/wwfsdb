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

    // Simple query to get user's accounts
    const accountsQuery = `SELECT account_id FROM accounts WHERE user_id = '${userId}'`;
    const userAccounts = await executeQuery<any[]>(accountsQuery);

    if (!userAccounts || userAccounts.length === 0) {
        return NextResponse.json({ success: true, transactions: [] });
    }

    // Simple approach: Get all transactions and filter in memory
    const query = `SELECT * FROM transactions ORDER BY created_at DESC`;
    const allTransactions = await executeQuery<any[]>(query);
    
    // Get user account IDs for filtering
    const userAccountIds = userAccounts.map(acc => acc.account_id);
    
    // Simple filtering - just check if transaction involves user's accounts
    const transactions = (allTransactions || []).filter(transaction => {
      const fromAccountId = transaction.from_account_id;
      const toAccountId = transaction.to_account_id;
      return userAccountIds.includes(fromAccountId) || userAccountIds.includes(toAccountId);
    });

    // Simple processing - convert amounts to numbers
    const processedTransactions = transactions.map(transaction => ({
      ...transaction,
      amount: typeof transaction.amount === 'string' 
        ? parseFloat(transaction.amount) 
        : Number(transaction.amount),
    }));

    return NextResponse.json({ success: true, transactions: processedTransactions });

  } catch (error) {
    console.error("[TRANSACTIONS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
