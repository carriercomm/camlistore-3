/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package com.danga.camli;

import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.ServiceConnection;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.os.IBinder;
import android.os.RemoteException;
import android.preference.CheckBoxPreference;
import android.preference.EditTextPreference;
import android.preference.Preference;
import android.preference.PreferenceActivity;
import android.preference.PreferenceScreen;
import android.preference.Preference.OnPreferenceChangeListener;
import android.util.Log;

public class SettingsActivity extends PreferenceActivity {
    private static final String TAG = "SettingsActivity";

    private IUploadService mServiceStub = null;

    private EditTextPreference hostPref;
    private EditTextPreference passwordPref;
    private CheckBoxPreference autoPref;
    private PreferenceScreen autoOpts;

    private SharedPreferences mSharedPrefs;
    private Preferences mPrefs;

    private final ServiceConnection mServiceConnection = new ServiceConnection() {
        public void onServiceConnected(ComponentName name, IBinder service) {
            mServiceStub = IUploadService.Stub.asInterface(service);
        }

        public void onServiceDisconnected(ComponentName name) {
            mServiceStub = null;
        };
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        getPreferenceManager().setSharedPreferencesName(Preferences.NAME);
        addPreferencesFromResource(R.xml.preferences);

        hostPref = (EditTextPreference) findPreference(Preferences.HOST);
        passwordPref = (EditTextPreference) findPreference(Preferences.PASSWORD);
        autoPref = (CheckBoxPreference) findPreference(Preferences.AUTO);
        autoOpts = (PreferenceScreen) findPreference(Preferences.AUTO_OPTS);

        mSharedPrefs = getSharedPreferences(Preferences.NAME, 0);
        mPrefs = new Preferences(mSharedPrefs);

        OnPreferenceChangeListener onChange = new OnPreferenceChangeListener() {
            public boolean onPreferenceChange(Preference pref, Object newValue) {
                final String key = pref.getKey();
                Log.v(TAG, "preference change for: " + key);

                // Note: newValue isn't yet persisted, but easiest to update the
                // UI here.
                String newStr = (newValue instanceof String) ? (String) newValue
                        : null;
                if (pref == hostPref) {
                    updateHostSummary(newStr);
                }
                if (pref == passwordPref) {
                    updatePasswordSummary(newStr);
                }
                return true; // yes, persist it
            }
        };
        hostPref.setOnPreferenceChangeListener(onChange);
        passwordPref.setOnPreferenceChangeListener(onChange);
    }

    private final SharedPreferences.OnSharedPreferenceChangeListener prefChangedHandler = new
        SharedPreferences.OnSharedPreferenceChangeListener() {
            public void onSharedPreferenceChanged(SharedPreferences sp, String key) {
                if (Preferences.AUTO.equals(key)) {
                    boolean val = mPrefs.autoUpload();
                    updateAutoOpts(val);
                    Log.d(TAG, "AUTO changed to " + val);
                    if (mServiceStub != null) {
                        try {
                            mServiceStub.setBackgroundWatchersEnabled(val);
                        } catch (RemoteException e) {
                            // Ignore.
                        }
                    }
                }

            }
        };

    @Override
    protected void onPause() {
        super.onPause();
        mSharedPrefs.unregisterOnSharedPreferenceChangeListener(prefChangedHandler);
        if (mServiceConnection != null) {
            unbindService(mServiceConnection);
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        updatePreferenceSummaries();
        mSharedPrefs.registerOnSharedPreferenceChangeListener(prefChangedHandler);
        bindService(new Intent(this, UploadService.class), mServiceConnection,
                Context.BIND_AUTO_CREATE);
    }

    private void updatePreferenceSummaries() {
        updateHostSummary(hostPref.getText());
        updatePasswordSummary(passwordPref.getText());
        updateAutoOpts(autoPref.isChecked());
    }

    private void updatePasswordSummary(String value) {
        if (value != null && value.length() > 0) {
            passwordPref.setSummary("*********");
        } else {
            passwordPref.setSummary("<unset>");
        }
    }

    private void updateHostSummary(String value) {
        if (value != null && value.length() > 0) {
            hostPref.setSummary(value);
        } else {
            hostPref.setSummary(getString(R.string.settings_host_summary));
        }
    }

    private void updateAutoOpts(boolean checked) {
        autoOpts.setEnabled(checked);
    }

    // Convenience method.
    static void show(Context context) {
        final Intent intent = new Intent(context, SettingsActivity.class);
        context.startActivity(intent);
    }
}
