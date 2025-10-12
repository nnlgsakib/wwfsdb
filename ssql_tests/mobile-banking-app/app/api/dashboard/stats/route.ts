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

    // 1. Get user's account IDs first
    const accountsQuery = `SELECT account_id FROM accounts WHERE user_id = '${userId}'`;
    const userAccounts = await executeQuery<any[]>(accountsQuery);

    if (!userAccounts || userAccounts.length === 0) {
        // If user has no accounts, return zeroed-out stats
        return NextResponse.json({ success: true, stats: {
            total_balance: 0,
            monthly_income: 0,
            monthly_expenses: 0,
            active_cards: 0,
            recent_transactions: [],
        } });
    }

    const accountIds = userAccounts.map(acc => `'${acc.account_id}'`).join(', ');
    const userAccountIdsQuery = `(${accountIds})`;

    const thirtyDaysAgo = new Date();
    thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
    const thirtyDaysAgoISO = thirtyDaysAgo.toISOString();

    // 2. Build queries using the fetched account IDs
    const queries = {
      total_balance: `SELECT SUM(balance) as total FROM accounts WHERE user_id = '${userId}';`,
      monthly_income: `SELECT SUM(amount) as total FROM transactions WHERE to_account_id IN ${userAccountIdsQuery} AND transaction_type = 'Deposit' AND created_at >= '${thirtyDaysAgoISO}';`,
      monthly_expenses: `SELECT SUM(amount) as total FROM transactions WHERE from_account_id IN ${userAccountIdsQuery} AND transaction_type IN ('Withdrawal', 'Transfer') AND created_at >= '${thirtyDaysAgoISO}';`,
      active_cards: `SELECT COUNT(*) as total FROM cards WHERE account_id IN ${userAccountIdsQuery} AND status = 'Active';`,
      recent_transactions: `SELECT * FROM transactions WHERE from_account_id IN ${userAccountIdsQuery} OR to_account_id IN ${userAccountIdsQuery} ORDER BY created_at DESC LIMIT 4;`,
    };

    // 3. Execute queries in parallel
    const [
        totalBalanceResult,
        monthlyIncomeResult,
        monthlyExpensesResult,
        activeCardsResult,
        recentTransactionsResult
    ] = await Promise.all([
        executeQuery<any[]>(queries.total_balance),
        executeQuery<any[]>(queries.monthly_income),
        executeQuery<any[]>(queries.monthly_expenses),
        executeQuery<any[]>(queries.active_cards),
        executeQuery<any[]>(queries.recent_transactions),
    ]);

    const stats = {
        total_balance: totalBalanceResult?.[0]?.total || 0,
        monthly_income: monthlyIncomeResult?.[0]?.total || 0,
        monthly_expenses: monthlyExpensesResult?.[0]?.total || 0,
        active_cards: activeCardsResult?.[0]?.total || 0,
        recent_transactions: recentTransactionsResult || [],
    };

    return NextResponse.json({ success: true, stats });

  } catch (error) {
    console.error("[DASHBOARD_STATS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
