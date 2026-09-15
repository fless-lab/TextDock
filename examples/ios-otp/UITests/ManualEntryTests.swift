import XCTest

final class ManualEntryTests: XCTestCase {
    @MainActor func testManualEntryAndRestartAfterCancellation() {
        let app = XCUIApplication()
        app.launch()
        let field = app.textFields["codeInput"]
        XCTAssertTrue(field.waitForExistence(timeout: 10))
        field.tap()
        field.typeText("482193")
        app.buttons["useCode"].tap()
        XCTAssertEqual(app.staticTexts["status"].label, "Six-digit input captured locally. No authentication request was sent.")
        app.buttons["newAttempt"].tap()
        app.buttons["cancelEntry"].tap()
        XCTAssertFalse(field.isEnabled)
        app.buttons["newAttempt"].tap()
        XCTAssertTrue(field.isEnabled)
        field.tap()
        field.typeText("000001")
        app.buttons["useCode"].tap()
        XCTAssertEqual(app.staticTexts["status"].label, "Six-digit input captured locally. No authentication request was sent.")
    }
}
