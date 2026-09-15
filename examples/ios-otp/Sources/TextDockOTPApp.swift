import SwiftUI

@main
struct TextDockOTPApp: App {
    var body: some Scene { WindowGroup { OTPView() } }
}

struct OTPView: View {
    @State private var attempt = OTPAttempt(now: Date())
    @State private var invalid = false
    @FocusState private var focused: Bool
    @Environment(\.scenePhase) private var scenePhase
    private let timer = Timer.publish(every: 1, on: .main, in: .common).autoconnect()

    private var status: String {
        switch attempt.state {
        case .entering:
            return invalid ? "Enter exactly six ASCII digits." : "Enter a code or choose an iOS suggestion. This local entry window lasts five minutes."
        case .expired: return "Entry window expired. Start a new attempt to enter a code."
        case .cancelled: return "Entry cancelled. Start a new attempt when ready."
        case .captured: return "Six-digit input captured locally. No authentication request was sent."
        }
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Text("This sample offers iOS one-time-code input. It does not read your SMS inbox, send an SMS or authenticate you.")
                    TextField("Verification code", text: $attempt.code)
                        .textContentType(.oneTimeCode)
                        .keyboardType(.numberPad)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                        .focused($focused)
                        .disabled(attempt.state != .entering)
                        .accessibilityIdentifier("codeInput")
                    Button("Use code (demo)") {
                        invalid = !attempt.capture(at: Date())
                        if !invalid { focused = false }
                    }
                    .disabled(attempt.state != .entering)
                    .accessibilityIdentifier("useCode")
                    Text(status).accessibilityIdentifier("status")
                } header: { Text("Verification code") }
                Section {
                    Button("New attempt") {
                        attempt = OTPAttempt(now: Date())
                        invalid = false
                        focused = true
                    }.accessibilityIdentifier("newAttempt")
                    Button("Cancel entry", role: .cancel) {
                        attempt.cancel()
                        invalid = false
                        focused = false
                    }.accessibilityIdentifier("cancelEntry")
                }
                Section("Testing") {
                    Text("Send ‘Your verification code is 482193’ to this iPhone, then focus the field. If iOS offers a code above the keyboard, choose it. Manual entry always works during an active attempt.")
                    Text("Simulator typing checks this form only. Native SMS suggestions require a physical receiving iPhone and platform support.")
                }
            }
            .navigationTitle("TextDock OTP")
            .onReceive(timer) { attempt.expire(at: $0) }
            .onChange(of: scenePhase) { phase in
                if phase == .active { attempt.expire(at: Date()) }
            }
            .onChange(of: attempt.state) { state in
                if state != .entering { focused = false }
            }
        }
    }
}
