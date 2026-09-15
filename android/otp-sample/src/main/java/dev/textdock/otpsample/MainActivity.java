package dev.textdock.otpsample;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.text.InputType;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import com.google.android.gms.auth.api.phone.SmsRetriever;
import com.google.android.gms.common.api.CommonStatusCodes;
import com.google.android.gms.common.api.Status;
import com.google.android.gms.tasks.Task;

/** Receiving example, deliberately separate from the SMS-sending gateway application. */
@SuppressWarnings("deprecation")
public final class MainActivity extends Activity {
    private static final int CONSENT_REQUEST = 17;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private final Otp.Window window = new Otp.Window();
    private BroadcastReceiver receiver;
    private EditText code;
    private TextView status;
    private long token;
    private boolean awaitingConsent;

    @Override public void onCreate(Bundle saved) {
        super.onCreate(saved);
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        int padding = (int) (24 * getResources().getDisplayMetrics().density);
        content.setPadding(padding, padding, padding, padding);
        ScrollView scroll = new ScrollView(this);
        scroll.setFillViewport(true);
        scroll.addView(content);
        setContentView(scroll);
        // Target 35 draws edge-to-edge. Keep controls outside system bars and keyboard.
        scroll.setOnApplyWindowInsetsListener((view, insets) -> {
            view.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(),
                insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
            return insets;
        });
        text(content, getString(R.string.title)).setTextSize(24);
        text(content, getString(R.string.intro));
        TextView label = text(content, getString(R.string.code));
        code = new EditText(this);
        code.setId(R.id.code_input);
        code.setSaveEnabled(false);
        code.setSingleLine(true);
        code.setInputType(InputType.TYPE_CLASS_NUMBER);
        code.setAutofillHints("smsOTPCode");
        label.setLabelFor(code.getId());
        content.addView(code);
        button(content, R.string.use_code, () -> {
            if (!Otp.valid(code.getText().toString())) { code.setError(getString(R.string.invalid)); return; }
            stop();
            code.setText("");
            status.setText(R.string.captured);
        });
        button(content, R.string.retriever, () -> start(false));
        button(content, R.string.consent, () -> start(true));
        button(content, R.string.cancel, () -> { stop(); status.setText(R.string.cancelled); });
        status = text(content, getString(saved == null ? R.string.manual : R.string.recreated));
        status.setId(R.id.status);
        status.setAccessibilityLiveRegion(View.ACCESSIBILITY_LIVE_REGION_POLITE);
        String hash = installedHash();
        TextView signing = text(content, hash == null ? getString(R.string.hash_unavailable) : getString(R.string.hash, hash));
        signing.setTextIsSelectable(true);
        if (hash != null) button(content, R.string.copy_sms, () -> {
            ClipboardManager clipboard = getSystemService(ClipboardManager.class);
            clipboard.setPrimaryClip(ClipData.newPlainText("TextDock test SMS", "<#> Your verification code is 482193\n" + hash));
            status.setText(R.string.copied);
        });
    }

    private TextView text(LinearLayout parent, String value) {
        TextView view = new TextView(this);
        view.setText(value);
        view.setTextSize(16);
        view.setPadding(0, 12, 0, 12);
        parent.addView(view);
        return view;
    }

    private void button(LinearLayout parent, int label, Runnable action) {
        Button button = new Button(this);
        button.setText(label);
        button.setAllCaps(false);
        button.setOnClickListener(view -> action.run());
        parent.addView(button);
    }

    private void start(boolean consent) {
        stop();
        token = window.start(SystemClock.elapsedRealtime());
        final long attempt = token;
        status.setText(R.string.starting);
        receiver = new BroadcastReceiver() {
            @Override public void onReceive(Context context, Intent intent) {
                if (!window.active(attempt, SystemClock.elapsedRealtime()) || awaitingConsent ||
                    !SmsRetriever.SMS_RETRIEVED_ACTION.equals(intent.getAction())) return;
                Status result = intent.getParcelableExtra(SmsRetriever.EXTRA_STATUS);
                if (result == null) return;
                if (result.getStatusCode() == CommonStatusCodes.TIMEOUT) {
                    stop(); status.setText(R.string.expired); return;
                }
                if (result.getStatusCode() != CommonStatusCodes.SUCCESS) {
                    stop(); status.setText(R.string.unavailable); return;
                }
                if (!consent) { received(intent.getStringExtra(SmsRetriever.EXTRA_SMS_MESSAGE)); return; }
                Intent prompt = intent.getParcelableExtra(SmsRetriever.EXTRA_CONSENT_INTENT);
                if (prompt == null) { stop(); status.setText(R.string.consent_denied); return; }
                try {
                    awaitingConsent = true;
                    status.setText(R.string.consent_pending);
                    startActivityForResult(prompt, CONSENT_REQUEST);
                } catch (RuntimeException unavailable) {
                    stop(); status.setText(R.string.consent_denied);
                }
            }
        };
        try {
            IntentFilter filter = new IntentFilter(SmsRetriever.SMS_RETRIEVED_ACTION);
            // Google Play services is external; require its signature permission on every broadcast.
            if (Build.VERSION.SDK_INT >= 33) registerReceiver(receiver, filter, SmsRetriever.SEND_PERMISSION, handler, Context.RECEIVER_EXPORTED);
            else registerReceiver(receiver, filter, SmsRetriever.SEND_PERMISSION, handler);
            Task<Void> task = consent ? SmsRetriever.getClient(this).startSmsUserConsent(null) : SmsRetriever.getClient(this).startSmsRetriever();
            task.addOnSuccessListener(ignored -> {
                if (window.active(attempt, SystemClock.elapsedRealtime()) && !awaitingConsent) status.setText(R.string.listening);
            });
            task.addOnFailureListener(error -> {
                if (window.active(attempt, SystemClock.elapsedRealtime())) { stop(); status.setText(R.string.unavailable); }
            });
            handler.postDelayed(() -> {
                // A cancelled/replaced window must not change the next attempt's UI.
                if (token == attempt) { stop(); status.setText(R.string.expired); }
            }, Otp.Window.DURATION);
        } catch (RuntimeException unavailable) {
            stop(); status.setText(R.string.unavailable);
        }
    }

    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request != CONSENT_REQUEST || !awaitingConsent || !window.active(token, SystemClock.elapsedRealtime())) return;
        if (result == RESULT_OK && data != null) received(data.getStringExtra(SmsRetriever.EXTRA_SMS_MESSAGE));
        else { stop(); status.setText(R.string.consent_denied); }
    }

    private void received(String message) {
        stop();
        String candidate = Otp.extract(message);
        if (candidate == null) status.setText(R.string.ambiguous);
        else if (code.length() != 0) status.setText(R.string.already_editing);
        else { code.setText(candidate); status.setText(R.string.received); }
    }

    private void stop() {
        window.cancel();
        token = 0;
        awaitingConsent = false;
        handler.removeCallbacksAndMessages(null);
        if (receiver != null) {
            try { unregisterReceiver(receiver); } catch (IllegalArgumentException notRegistered) { /* Registration failed. */ }
            receiver = null;
        }
    }

    @Override protected void onStop() {
        super.onStop();
        // The Google consent activity temporarily covers this activity; keep that attempt alive.
        if (!awaitingConsent && token != 0) { stop(); status.setText(R.string.cancelled); }
    }

    @Override protected void onDestroy() { stop(); super.onDestroy(); }

    private String installedHash() {
        try {
            PackageInfo info = getPackageManager().getPackageInfo(getPackageName(),
                Build.VERSION.SDK_INT >= 28 ? PackageManager.GET_SIGNING_CERTIFICATES : PackageManager.GET_SIGNATURES);
            Signature[] signatures = Build.VERSION.SDK_INT >= 28 ? info.signingInfo.getApkContentsSigners() : info.signatures;
            return signatures.length == 1 ? Otp.appHash(getPackageName(), signatures[0].toCharsString()) : null;
        } catch (PackageManager.NameNotFoundException | RuntimeException unavailable) { return null; }
    }
}
