package dev.textdock.gateway;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import org.json.JSONObject;

public final class SmsSentReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context, Intent intent) {
        SharedPreferences prefs = context.getSharedPreferences("gateway", Context.MODE_PRIVATE);
        try {
            if (!prefs.contains("job")) return;
            JSONObject job = new JSONObject(prefs.getString("job", "{}"));
            if (!job.getString("id").equals(intent.getStringExtra("job"))) return;
            JSONObject parts = new JSONObject(prefs.getString("parts", "{}"));
            parts.put(Integer.toString(intent.getIntExtra("part", -1)), getResultCode() == Activity.RESULT_OK);
            SharedPreferences.Editor edit = prefs.edit().putString("parts", parts.toString());
            if (parts.length() == prefs.getInt("total", 1)) {
                int successful = 0;
                for (java.util.Iterator<String> keys = parts.keys(); keys.hasNext();) if (parts.getBoolean(keys.next())) successful++;
                String state = successful == parts.length() ? "sent" : successful == 0 ? "failed" : "unknown";
                edit.putString("phase", "ack").putString("result", state).putString("error", state.equals("sent") ? "" : "One or more SMS segments failed");
            }
            edit.commit();
        } catch (Exception ignored) { prefs.edit().putString("status", "Unable to record SMS result; inspect the server relay state.").apply(); }
    }
}
