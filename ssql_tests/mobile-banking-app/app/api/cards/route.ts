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
        return NextResponse.json({ success: true, cards: [] });
    }

    const accountIds = userAccounts.map(acc => `'${acc.account_id}'`).join(', ');

    const query = `
      SELECT * FROM cards 
      WHERE account_id IN (${accountIds}) 
      ORDER BY created_at DESC`;
      
    const cards = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, cards: cards || [] });

  } catch (error) {
    console.error("[CARDS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
