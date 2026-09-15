package dev.textdock.otpsample;

import androidx.test.ext.junit.rules.ActivityScenarioRule;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import static androidx.test.espresso.Espresso.onView;
import static androidx.test.espresso.action.ViewActions.*;
import static androidx.test.espresso.assertion.ViewAssertions.matches;
import static androidx.test.espresso.matcher.ViewMatchers.*;

/** UI flow checks; these do not synthesize a Play services result or claim SMS autofill. */
@RunWith(AndroidJUnit4.class)
public class ManualEntryTest {
    @Rule public ActivityScenarioRule<MainActivity> activity = new ActivityScenarioRule<>(MainActivity.class);

    @Test public void manualEntryWorksWithoutStartingSmsListening() {
        onView(withId(R.id.code_input)).perform(replaceText("482193"), closeSoftKeyboard());
        onView(withText(R.string.use_code)).perform(scrollTo(), click());
        onView(withId(R.id.status)).check(matches(withText(R.string.captured)));
        onView(withId(R.id.code_input)).check(matches(withText("")));
    }

    @Test public void cancellationAndInvalidInputKeepManualFallback() {
        onView(withText(R.string.cancel)).perform(scrollTo(), click());
        onView(withId(R.id.status)).check(matches(withText(R.string.cancelled)));
        onView(withId(R.id.code_input)).perform(scrollTo(), replaceText("1234567"), closeSoftKeyboard());
        onView(withText(R.string.use_code)).perform(scrollTo(), click());
        onView(withId(R.id.code_input)).check(matches(hasErrorText("Enter exactly six ASCII digits.")));
        activity.getScenario().recreate();
        onView(withId(R.id.code_input)).check(matches(withText("")));
        onView(withId(R.id.status)).check(matches(withText(R.string.recreated)));
    }
}
