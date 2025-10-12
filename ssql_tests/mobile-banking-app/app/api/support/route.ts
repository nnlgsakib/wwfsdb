import { type NextRequest, NextResponse } from "next/server";
import { executeQuery, executeWriteQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";
import { v4 as uuidv4 } from "uuid";

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const query = `SELECT * FROM support_tickets WHERE user_id = '${userId}' ORDER BY created_at DESC`;
    const tickets = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, tickets: tickets || [] });

  } catch (error) {
    console.error("[SUPPORT_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}

export async function POST(request: NextRequest) {
    try {
        const userId = getUserIdFromRequest(request);
        if (!userId) {
          return NextResponse.json(
            { success: false, message: "Unauthorized" },
            { status: 401 }
          );
        }

        const { subject, description } = await request.json();

        if (!subject || !description) {
            return NextResponse.json(
                { success: false, message: "Subject and description are required" },
                { status: 400 }
            );
        }

        const now = new Date().toISOString();
        const newTicket = {
            ticket_id: uuidv4(),
            user_id: userId,
            subject,
            description,
            status: "Open",
            created_at: now,
            updated_at: now,
        };

        const insertQuery = `
            INSERT INTO support_tickets (ticket_id, user_id, subject, description, status, created_at, updated_at)
            VALUES ('${newTicket.ticket_id}', '${newTicket.user_id}', '${newTicket.subject}', '${newTicket.description}', '${newTicket.status}', '${newTicket.created_at}', '${newTicket.updated_at}');
        `;

        await executeWriteQuery(insertQuery);

        return NextResponse.json({
            success: true,
            ticket: newTicket,
            message: "Support ticket created successfully",
        });

    } catch (error) {
        console.error("[SUPPORT_POST_ERROR]", error);
        return NextResponse.json(
            { success: false, message: "Server error" },
            { status: 500 }
        );
    }
}
