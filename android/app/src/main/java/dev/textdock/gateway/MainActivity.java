package dev.textdock.gateway;

import android.Manifest;
import android.app.Activity;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Spinner;
import android.widget.ArrayAdapter;
import android.telephony.SubscriptionInfo;
import android.telephony.SubscriptionManager;
import java.net.URI;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

public final class MainActivity extends Activity {
    private EditText address, token;
    private TextView status;
    private Spinner sim;
    private final ArrayList<Integer> subscriptions = new ArrayList<>();
    private final Handler handler = new Handler(Looper.getMainLooper());
    private final Runnable refresh = new Runnable() {
        public void run() { status.setText(prefs().getString("status", "Stopped")); handler.postDelayed(this, 1000); }
    };
    private SharedPreferences prefs() { return getSharedPreferences("gateway", MODE_PRIVATE); }
    private TextView text(String value, int size) {
        TextView view = new TextView(this); view.setText(value); view.setTextSize(size); view.setPadding(0, 14, 0, 10); return view;
    }
    @Override public void onCreate(Bundle saved) {
        super.onCreate(saved);
        LinearLayout layout = new LinearLayout(this); layout.setOrientation(LinearLayout.VERTICAL);
        int margin = (int) (24 * getResources().getDisplayMetrics().density); layout.setPadding(margin, margin, margin, margin);
        layout.addView(text("TextDock Gateway", 24));
        layout.addView(text("Development companion. Sends real SMS through the selected SIM, or Android’s default SMS SIM.", 15));
        layout.addView(text("Server address", 14));
        address = new EditText(this); address.setSingleLine(true); address.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        address.setHint("http://192.168.1.10:18257"); address.setText(prefs().getString("url", "")); layout.addView(address);
        layout.addView(text("Gateway token", 14));
        token = new EditText(this); token.setSingleLine(true); token.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        token.setHint("Token from TextDock → Relay"); token.setText(prefs().getString("token", "")); layout.addView(token);
        layout.addView(text("Sending SIM", 14));
        sim = new Spinner(this); layout.addView(sim); loadSIMs();
        Button refreshSIM = new Button(this); refreshSIM.setText("Refresh SIM list"); refreshSIM.setOnClickListener(v -> {
            if (checkSelfPermission(Manifest.permission.READ_PHONE_STATE) != PackageManager.PERMISSION_GRANTED) requestPermissions(new String[]{Manifest.permission.READ_PHONE_STATE}, 3);
            else loadSIMs();
        }); layout.addView(refreshSIM);
        Button start = new Button(this); start.setText("Start gateway"); start.setOnClickListener(v -> start()); layout.addView(start);
        Button stop = new Button(this); stop.setText("Stop gateway"); stop.setOnClickListener(v -> {
            stopService(new Intent(this, GatewayService.class)); prefs().edit().putString("status", "Stopped").apply();
        }); layout.addView(stop);
        Button reset = new Button(this); reset.setText("Reset connection"); reset.setOnClickListener(v -> {
            synchronized (GatewayState.LOCK) { prefs().edit().clear().putString("session", UUID.randomUUID().toString()).putString("status", "Connection reset. Pending server jobs are not resent.").commit(); }
            stopService(new Intent(this, GatewayService.class)); address.setText(""); token.setText(""); loadSIMs();
        }); layout.addView(reset);
        status = text("Stopped", 14); status.setTextIsSelectable(true); layout.addView(status);
        layout.addView(text("Create a gateway credential on the desktop. This credential can claim relay jobs; it cannot open the desktop inbox. Stopping does not undo an SMS already submitted to the network.", 14));
        ScrollView scroll = new ScrollView(this); scroll.addView(layout); setContentView(scroll);
    }
    private void start() {
        synchronized (GatewayState.LOCK) { startConfigured(); }
    }
    private void startConfigured() {
        String url = address.getText().toString().trim().replaceAll("/+$", "");
        String secret = token.getText().toString().trim();
        int subscription = subscriptions.get(sim.getSelectedItemPosition());
        try {
            URI uri = URI.create(url);
            if ((!"http".equals(uri.getScheme()) && !"https".equals(uri.getScheme())) || uri.getHost() == null || uri.getUserInfo() != null || uri.getQuery() != null || uri.getFragment() != null || (!uri.getPath().isEmpty() && !"/".equals(uri.getPath()))) throw new IllegalArgumentException();
            if (!secret.startsWith("td_gateway_")) throw new IllegalArgumentException();
        } catch (Exception error) { prefs().edit().putString("status", "Enter a server origin and a valid gateway token.").apply(); return; }
        boolean changed = !url.equals(prefs().getString("url", "")) || !secret.equals(prefs().getString("token", "")) || subscription != prefs().getInt("subscription_id", -1);
        if (changed && prefs().contains("job")) { prefs().edit().putString("status", "A result is pending. Inspect it on the server before resetting or changing the connection/SIM.").apply(); return; }
        if (subscription >= 0) {
            if (checkSelfPermission(Manifest.permission.READ_PHONE_STATE) != PackageManager.PERMISSION_GRANTED) { prefs().edit().putString("status", "Refresh the SIM list to grant phone-state permission, or choose the default SIM.").apply(); return; }
            if (getSystemService(SubscriptionManager.class).getActiveSubscriptionInfo(subscription) == null) { prefs().edit().putString("status", "Selected SIM is not active. Choose another SIM explicitly.").apply(); return; }
        }
        SharedPreferences.Editor edit = prefs().edit().putString("url", url).putString("token", secret).putInt("subscription_id", subscription);
        if (changed || !prefs().contains("session")) edit.putString("session", UUID.randomUUID().toString());
        edit.commit();
        if (checkSelfPermission(Manifest.permission.SEND_SMS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.SEND_SMS}, 1); return;
        }
        if (android.os.Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 2);
        if (changed) { stopService(new Intent(this, GatewayService.class)); handler.postDelayed(() -> startForegroundService(new Intent(this, GatewayService.class)), 250); }
        else startForegroundService(new Intent(this, GatewayService.class));
    }
    private void loadSIMs() {
        ArrayList<String> labels = new ArrayList<>(); subscriptions.clear();
        labels.add("Android default SMS SIM"); subscriptions.add(-1);
        int saved = prefs().getInt("subscription_id", -1);
        if (checkSelfPermission(Manifest.permission.READ_PHONE_STATE) == PackageManager.PERMISSION_GRANTED) {
            List<SubscriptionInfo> active = getSystemService(SubscriptionManager.class).getActiveSubscriptionInfoList();
            if (active != null) for (SubscriptionInfo item : active) { subscriptions.add(item.getSubscriptionId()); labels.add("SIM " + (item.getSimSlotIndex() + 1) + " · " + item.getCarrierName()); }
        }
        if (saved >= 0 && !subscriptions.contains(saved)) { subscriptions.add(saved); labels.add("Saved subscription " + saved + " (refresh to verify)"); }
        sim.setAdapter(new ArrayAdapter<>(this, android.R.layout.simple_spinner_dropdown_item, labels));
        sim.setSelection(subscriptions.indexOf(saved));
    }
    @Override public void onRequestPermissionsResult(int request, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(request, permissions, results);
        if (request == 1) {
            if (results.length > 0 && results[0] == PackageManager.PERMISSION_GRANTED) start();
            else prefs().edit().putString("status", "SMS permission is required to start the gateway.").apply();
        }
        if (request == 3) { loadSIMs(); prefs().edit().putString("status", "SIM list updated. Default-SIM sending does not require phone-state access.").apply(); }
    }
    @Override protected void onResume() { super.onResume(); handler.post(refresh); }
    @Override protected void onPause() { handler.removeCallbacks(refresh); super.onPause(); }
}
