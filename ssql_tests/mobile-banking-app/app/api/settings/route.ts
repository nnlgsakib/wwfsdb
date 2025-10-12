import { type NextRequest, NextResponse } from "next/server";
import { executeQuery, executeWriteQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";
import { v4 as uuidv4 } from 'uuid';

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const query = `SELECT * FROM app_settings WHERE user_id = '${userId}'`;
    const settings = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, settings: settings || [] });

  } catch (error) {
    console.error("[SETTINGS_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}

export async function PUT(request: NextRequest) {
    try {
        const userId = getUserIdFromRequest(request);
        if (!userId) {
          return NextResponse.json(
            { success: false, message: "Unauthorized" },
            { status: 401 }
          );
        }

        const { setting_name, setting_value } = await request.json();

        if (!setting_name || typeof setting_value === 'undefined') {
            return NextResponse.json(
                { success: false, message: "Setting name and value are required" },
                { status: 400 }
            );
        }

        const setting_id = uuidv4();

        // Use a transaction to perform a clean upsert (DELETE then INSERT)
        const upsertQuery = [
            `BEGIN;`,
            `DELETE FROM app_settings WHERE user_id = '${userId}' AND setting_name = '${setting_name}';`,
            `INSERT INTO app_settings (setting_id, user_id, setting_name, setting_value) VALUES ('${setting_id}', '${userId}', '${setting_name}', '${setting_value}');`,
            `COMMIT;`
        ].join('\n');

        await executeWriteQuery(upsertQuery);

        return NextResponse.json({
            success: true,
            message: "Setting updated successfully",
        });

    } catch (error) {
        console.error("[SETTINGS_PUT_ERROR]", error);
        await executeWriteQuery('ROLLBACK;');
        return NextResponse.json(
            { success: false, message: "Server error" },
            { status: 500 }
        );
    }
}
