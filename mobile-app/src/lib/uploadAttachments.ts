import * as FileSystem from "expo-file-system/legacy";
import { ApiError } from "../api/errors";
import { messageFrom } from "../api/client";
import { getSessionToken } from "../auth/session";
import { appExtra } from "../api/runtime";

type LocalFile = { name: string; uri: string; mime?: string | null };

function parseBody(body: string): unknown {
  try {
    return body ? JSON.parse(body) : null;
  } catch {
    return { message: body };
  }
}

/** Upload local files via native multipart (RN FormData + fetch is unreliable on Android). */
export async function uploadTransactionAttachments(
  entityId: string,
  transactionId: string,
  files: LocalFile[],
): Promise<void> {
  const base = (appExtra().apiBaseUrl || "").replace(/\/$/, "");
  const token = await getSessionToken();
  for (const file of files) {
    const result = await FileSystem.uploadAsync(
      `${base}/entities/${entityId}/transactions/${transactionId}/attachments`,
      file.uri,
      {
        httpMethod: "POST",
        uploadType: FileSystem.FileSystemUploadType.MULTIPART,
        fieldName: "files",
        mimeType: file.mime || "application/octet-stream",
        headers: {
          Accept: "application/json",
          "X-Requested-With": "MyanKafe-Finance",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      },
    );
    if (result.status < 200 || result.status >= 300) {
      throw new ApiError(messageFrom(parseBody(result.body), result.status), result.status);
    }
  }
}
