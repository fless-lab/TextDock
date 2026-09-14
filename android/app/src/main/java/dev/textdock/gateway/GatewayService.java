package dev.textdock.gateway;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.Manifest;
import android.content.pm.PackageManager;
import android.content.Intent;
import android.content.SharedPreferences;
import android.os.IBinder;
import android.net.Uri;
import android.telephony.SmsManager;
import android.telephony.SubscriptionManager;
import android.os.Build;
import org.json.JSONObject;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public final class GatewayService extends Service {
    private static final Object POLL_LOCK = new Object();
    private volatile boolean stopping;
    private ScheduledExecutorService executor;
    private SharedPreferences prefs;
    private String session, serverURL, credential, appVersion;
    private long heartbeat;
    @Override public void onCreate() {
        super.onCreate(); prefs = getSharedPreferences("gateway", MODE_PRIVATE);
        session = prefs.getString("session", ""); serverURL = prefs.getString("url", ""); credential = prefs.getString("token", "");
        try { appVersion = getPackageManager().getPackageInfo(getPackageName(), 0).versionName; } catch (Exception error) { appVersion = "unknown"; }
        NotificationManager manager = getSystemService(NotificationManager.class);
        manager.createNotificationChannel(new NotificationChannel("relay", "SMS relay", NotificationManager.IMPORTANCE_LOW));
        Notification notification = new Notification.Builder(this, "relay").setContentTitle("TextDock gateway running").setContentText("Waiting for relay jobs · real SMS via SIM").setSmallIcon(android.R.drawable.ic_dialog_email).setOngoing(true).build();
        startForeground(1, notification);
        // Never repeat a carrier call after process/service recreation.
        if (prefs.contains("job") && !prefs.getString("phase", "").equals("ack")) prefs.edit().putString("phase", "ack").putString("result", "unknown").putString("error", "Gateway restarted during dispatch").commit();
        executor = Executors.newSingleThreadScheduledExecutor();
        executor.scheduleWithFixedDelay(this::poll, 0, 2, TimeUnit.SECONDS);
    }
    @Override public int onStartCommand(Intent intent, int flags, int startId) { if (!currentSession()) stopSelf(); return START_NOT_STICKY; }
    private boolean currentSession() { return session.equals(prefs.getString("session", "")); }
    private void status(String value) { synchronized (GatewayState.LOCK) { if (currentSession()) prefs.edit().putString("status", value).apply(); } }
    private JSONObject post(String path, JSONObject data) throws Exception {
        if (!currentSession()) throw new Exception("session_changed");
        HttpURLConnection connection = (HttpURLConnection) new URL(serverURL + path).openConnection();
        connection.setConnectTimeout(8000); connection.setReadTimeout(8000); connection.setInstanceFollowRedirects(false);
        connection.setRequestMethod("POST"); connection.setRequestProperty("Authorization", "Bearer " + credential);
        connection.setRequestProperty("Content-Type", "application/json"); connection.setDoOutput(true);
        try {
            byte[] body = (data == null ? "{}" : data.toString()).getBytes(StandardCharsets.UTF_8);
            try (java.io.OutputStream output = connection.getOutputStream()) { output.write(body); }
            int code = connection.getResponseCode();
            if (!currentSession()) throw new Exception("session_changed");
            if (code == 401 || code == 403) { status("Authorization rejected. Check the server driver and gateway credential."); stopSelf(); throw new Exception("authorization"); }
            if (code == 204) return null;
            if (code == 404 && path.equals("/relay/v1/heartbeat")) return null; // Older servers have no health endpoint.
            if (code == 409 && path.equals("/relay/v1/jobs/result")) throw new Exception("result_conflict");
            if (code != 200) throw new Exception("Server HTTP " + code);
            try (InputStream input = connection.getInputStream()) {
                java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream(); byte[] buffer = new byte[4096]; int count;
                while ((count = input.read(buffer)) != -1) { if (out.size() + count > 65536) throw new Exception("Response too large"); out.write(buffer, 0, count); }
                return new JSONObject(out.toString("UTF-8"));
            }
        } finally { connection.disconnect(); }
    }
    private void poll() {
        synchronized (POLL_LOCK) { if (!stopping) pollOnce(); }
    }
    private void pollOnce() {
        try {
            if (!currentSession()) { stopSelf(); return; }
            if (System.currentTimeMillis() - heartbeat >= 15000) {
                String model = Build.MANUFACTURER + " " + Build.MODEL;
                if (model.length() > 40) model = model.substring(0, 40);
                try { post("/relay/v1/heartbeat", new JSONObject().put("model", model).put("app_version", appVersion).put("subscription_id", prefs.getInt("subscription_id", -1))); }
                catch (Exception error) { if ("authorization".equals(error.getMessage()) || "session_changed".equals(error.getMessage())) throw error; }
                heartbeat = System.currentTimeMillis();
            }
            JSONObject result = null;
            synchronized (GatewayState.LOCK) {
                if (!currentSession()) return;
                if (prefs.contains("job")) {
                    if (!prefs.getString("phase", "").equals("ack")) {
                        if (System.currentTimeMillis() - prefs.getLong("started", 0) > 45000) prefs.edit().putString("phase", "ack").putString("result", "unknown").putString("error", "SMS result timed out").commit();
                        else return;
                    }
                    JSONObject job = new JSONObject(prefs.getString("job", "{}"));
                    result = new JSONObject().put("id", job.getString("id")).put("lease_token", job.getString("lease_token")).put("state", prefs.getString("result", "unknown")).put("error", prefs.getString("error", ""));
                }
            }
            if (result != null) {
                post("/relay/v1/jobs/result", result);
                synchronized (GatewayState.LOCK) { if (!currentSession()) return; prefs.edit().remove("job").remove("phase").remove("parts").remove("result").remove("error").commit(); }
                status("Last SMS: " + result.getString("state")); return;
            }
            JSONObject job = post("/relay/v1/jobs/claim", null);
            if (job == null) return;
            if (!currentSession()) return;
            int subscription = prefs.getInt("subscription_id", -1);
            boolean selectedSIMValid = true;
            if (subscription >= 0) {
                if (checkSelfPermission(Manifest.permission.READ_PHONE_STATE) != PackageManager.PERMISSION_GRANTED) selectedSIMValid = false;
                else selectedSIMValid = getSystemService(SubscriptionManager.class).getActiveSubscriptionInfo(subscription) != null;
            }
            SmsManager manager = subscription < 0 ? SmsManager.getDefault() : SmsManager.getSmsManagerForSubscriptionId(subscription);
            ArrayList<String> parts = manager.divideMessage(job.getString("body"));
            synchronized (GatewayState.LOCK) { if (!currentSession()) return; prefs.edit().putString("job", job.toString()).putString("phase", "dispatching").putLong("started", System.currentTimeMillis()).putInt("total", parts.size()).putString("parts", "{}").commit(); }
            if (!selectedSIMValid) { prefs.edit().putString("phase", "ack").putString("result", "failed").putString("error", "Selected SIM is unavailable or phone-state permission was revoked").commit(); return; }
            if (stopping) { prefs.edit().putString("phase", "ack").putString("result", "unknown").putString("error", "Gateway stopped after claiming a job").commit(); return; }
            ArrayList<PendingIntent> sent = new ArrayList<>();
            for (int i = 0; i < parts.size(); i++) {
                Intent intent = new Intent(this, SmsSentReceiver.class).setAction("dev.textdock.gateway.SENT").setData(Uri.parse("textdock-sms://sent/" + job.getString("id") + "/" + i)).putExtra("job", job.getString("id")).putExtra("part", i);
                sent.add(PendingIntent.getBroadcast(this, (job.getString("id") + ":" + i).hashCode(), intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE));
            }
            status("Sending " + job.getString("id") + " via SIM");
            if (checkSelfPermission(Manifest.permission.SEND_SMS) != PackageManager.PERMISSION_GRANTED) {
                prefs.edit().putString("phase", "ack").putString("result", "failed").putString("error", "SMS permission is not granted").commit(); return;
            }
            if (!currentSession()) return;
            try { manager.sendMultipartTextMessage(job.getString("to"), null, parts, sent, null); }
            catch (Exception error) { prefs.edit().putString("phase", "ack").putString("result", "unknown").putString("error", "SMS submission interrupted").commit(); }
        } catch (Exception error) {
            if ("session_changed".equals(error.getMessage())) { stopSelf(); return; }
            if ("result_conflict".equals(error.getMessage())) {
                synchronized (GatewayState.LOCK) { if (!currentSession()) return; prefs.edit().remove("job").remove("phase").remove("parts").remove("result").remove("error").commit(); }
                status("Result no longer accepted by server. Inspect its history; this job will not be resent.");
            } else if (!"authorization".equals(error.getMessage())) status("Waiting for server: " + error.getMessage());
        }
    }
    @Override public void onDestroy() { stopping = true; if (executor != null) executor.shutdownNow(); super.onDestroy(); }
    @Override public IBinder onBind(Intent intent) { return null; }
}
