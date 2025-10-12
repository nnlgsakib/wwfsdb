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

    const query = `SELECT * FROM accounts WHERE user_id = '${userId}' ORDER BY created_at DESC`;
    const accounts = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, accounts: accounts || [] });

  } catch (error) {
    console.error("[ACCOUNTS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
