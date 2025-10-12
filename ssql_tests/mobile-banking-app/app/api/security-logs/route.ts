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

    const query = `SELECT * FROM security_logs WHERE user_id = '${userId}' ORDER BY timestamp DESC`;
    const logs = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, logs: logs || [] });

  } catch (error) {
    console.error("[SECURITY_LOGS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
