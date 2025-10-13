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

    const query = `
      SELECT 
        account_id,
        account_number,
        account_type,
        balance,
        currency,
        status,
        created_at
      FROM accounts 
      WHERE user_id = '${userId}' 
      ORDER BY created_at DESC`;
    const accounts = await executeQuery<any[]>(query);

    // Ensure numeric fields are properly typed and handle both qualified/unqualified field names
    const processedAccounts = (accounts || []).map(account => {
      // Handle both cases: simple field names and qualified field names (e.g. "accounts.balance")
      // First, let's determine the prefix (if any) by checking if any account field starts with "accounts."
      const hasPrefix = Object.keys(account).some(key => key.startsWith("accounts."));
      let processedAccount;
      
      if (hasPrefix) {
        // If fields have the "accounts." prefix, use those values
        processedAccount = {
          account_id: account["accounts.account_id"],
          account_number: account["accounts.account_number"],
          account_type: account["accounts.account_type"],
          balance: account["accounts.balance"],
          currency: account["accounts.currency"],
          status: account["accounts.status"],
          created_at: account["accounts.created_at"],
          user_id: account["accounts.user_id"],
        };
      } else {
        // If fields don't have the prefix, use the regular names
        processedAccount = {
          account_id: account.account_id,
          account_number: account.account_number,
          account_type: account.account_type,
          balance: account.balance,
          currency: account.currency,
          status: account.status,
          created_at: account.created_at,
          user_id: account.user_id,
        };
      }

      // Convert numeric fields
      processedAccount.balance = typeof processedAccount.balance === 'string' 
        ? parseFloat(processedAccount.balance) 
        : Number(processedAccount.balance);

      return processedAccount;
    });

    return NextResponse.json({ success: true, accounts: processedAccounts });

  } catch (error) {
    console.error("[ACCOUNTS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
