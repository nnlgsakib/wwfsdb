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

    let responseStats = {
        total_balance: 0,
        monthly_income: 0,
        monthly_expenses: 0,
        active_cards: 0,
        recent_transactions: [],
    };

    if (userAccounts && userAccounts.length > 0) {
        const accountIds = userAccounts.map(acc => `'${acc.account_id}'`).join(', ');

        // Only proceed if we have valid account IDs
        if (accountIds && accountIds !== "''") { 
            const userAccountIdsQuery = `(${accountIds})`;
            const thirtyDaysAgo = new Date();
            thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
            const thirtyDaysAgoISO = thirtyDaysAgo.toISOString();

            // 2. Build queries using the fetched account IDs
            const queries = {
            total_balance: `SELECT SUM(balance) as total FROM accounts WHERE account_id IN ${userAccountIdsQuery};`,
            monthly_income: `SELECT SUM(amount) as total FROM transactions WHERE to_account_id IN ${userAccountIdsQuery} AND transaction_type = 'Deposit' AND created_at >= '${thirtyDaysAgoISO}';`,
            monthly_expenses: `SELECT SUM(amount) as total FROM transactions WHERE from_account_id IN ${userAccountIdsQuery} AND transaction_type IN ('Withdrawal', 'Transfer') AND created_at >= '${thirtyDaysAgoISO}';`,
            active_cards: `SELECT COUNT(*) as total FROM cards WHERE account_id IN ${userAccountIdsQuery} AND status = 'Active';`,
            recent_transactions: `SELECT 
              transaction_id,
              from_account_id,
              to_account_id,
              amount,
              currency,
              transaction_type,
              description,
              status,
              created_at
            FROM transactions 
            ORDER BY created_at DESC 
            LIMIT 100;`,
            };

            // 3. Execute queries in parallel
            try {
                const [
                    totalBalanceResult,
                    monthlyIncomeResult,
                    monthlyExpensesResult,
                    activeCardsResult,
                ] = await Promise.all([
                    executeQuery<any[]>(queries.total_balance),
                    executeQuery<any[]>(queries.monthly_income),
                    executeQuery<any[]>(queries.monthly_expenses),
                    executeQuery<any[]>(queries.active_cards),
                ]);

                const allRecentTransactions = await executeQuery<any[]>(queries.recent_transactions);
                
                // Filter transactions to only include those related to user's accounts
                const userTransactionIds = userAccounts.map(acc => acc.account_id);
                const recentTransactionsResult = (allRecentTransactions || []).filter(transaction => {
                  const fromAccountId = transaction.from_account_id || transaction["transactions.from_account_id"];
                  const toAccountId = transaction.to_account_id || transaction["transactions.to_account_id"];
                  return userTransactionIds.includes(fromAccountId) || userTransactionIds.includes(toAccountId);
                }).slice(0, 4); // Limit to 4 as originally intended

                responseStats = {
                    total_balance: typeof (totalBalanceResult?.[0]?.total) === 'string' ? parseFloat(totalBalanceResult?.[0]?.total) : Number(totalBalanceResult?.[0]?.total || 0),
                    monthly_income: typeof (monthlyIncomeResult?.[0]?.total) === 'string' ? parseFloat(monthlyIncomeResult?.[0]?.total) : Number(monthlyIncomeResult?.[0]?.total || 0),
                    monthly_expenses: typeof (monthlyExpensesResult?.[0]?.total) === 'string' ? parseFloat(monthlyExpensesResult?.[0]?.total) : Number(monthlyExpensesResult?.[0]?.total || 0),
                    active_cards: typeof (activeCardsResult?.[0]?.total) === 'string' ? parseInt(activeCardsResult?.[0]?.total) : Number(activeCardsResult?.[0]?.total || 0),
                    recent_transactions: (recentTransactionsResult || []).map(transaction => {
                      // Handle both cases: simple field names and qualified field names
                      // First, let's determine the prefix (if any) by checking if any transaction field starts with "transactions."
                      const hasPrefix = Object.keys(transaction).some(key => key.startsWith("transactions."));
                      let processedTransaction;
                      
                      if (hasPrefix) {
                        // If fields have the "transactions." prefix, use those values
                        processedTransaction = {
                          transaction_id: transaction["transactions.transaction_id"],
                          from_account_id: transaction["transactions.from_account_id"],
                          to_account_id: transaction["transactions.to_account_id"],
                          amount: transaction["transactions.amount"],
                          currency: transaction["transactions.currency"],
                          transaction_type: transaction["transactions.transaction_type"],
                          description: transaction["transactions.description"],
                          status: transaction["transactions.status"],
                          created_at: transaction["transactions.created_at"],
                        };
                      } else {
                        // If fields don't have the prefix, use the regular names
                        processedTransaction = {
                          transaction_id: transaction.transaction_id,
                          from_account_id: transaction.from_account_id,
                          to_account_id: transaction.to_account_id,
                          amount: transaction.amount,
                          currency: transaction.currency,
                          transaction_type: transaction.transaction_type,
                          description: transaction.description,
                          status: transaction.status,
                          created_at: transaction.created_at,
                        };
                      }

                      // Convert numeric fields
                      processedTransaction.amount = typeof processedTransaction.amount === 'string' 
                        ? parseFloat(processedTransaction.amount) 
                        : Number(processedTransaction.amount);

                      return processedTransaction;
                    }),
                };
            } catch (queryError) {
                console.error("[DASHBOARD_STATS_QUERY_ERROR]", queryError);
                // Keep the default zeroed-out stats in case of query errors
            }
        }
    }

    return NextResponse.json({ success: true, stats: responseStats });

  } catch (error) {
    console.error("[DASHBOARD_STATS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
