package com.arko.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.biometric.BiometricPrompt
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.fragment.app.FragmentActivity
import mobile.NativeBridge
import java.util.concurrent.CountDownLatch

class NativeBridgeImpl(
    private val activity: FragmentActivity,
    private val context: Context,
) : NativeBridge {

    override fun getDeviceID(): String =
        Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)

    override fun showNotification(title: String, body: String) {
        val notification = NotificationCompat.Builder(context, "arko_default")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle(title)
            .setContentText(body)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .build()

        NotificationManagerCompat.from(context).notify(System.currentTimeMillis().toInt(), notification)
    }

    override fun authenticateBiometric(reason: String): Boolean {
        val latch = CountDownLatch(1)
        var result = false

        activity.runOnUiThread {
            val executor = context.mainExecutor
            val prompt = BiometricPrompt(activity, executor, object : BiometricPrompt.AuthenticationCallback() {
                override fun onAuthenticationSucceeded(r: BiometricPrompt.AuthenticationResult) {
                    result = true
                    latch.countDown()
                }
                override fun onAuthenticationError(code: Int, msg: CharSequence) { latch.countDown() }
                override fun onAuthenticationFailed() { latch.countDown() }
            })

            val info = BiometricPrompt.PromptInfo.Builder()
                .setTitle("Authenticate")
                .setSubtitle(reason)
                .setNegativeButtonText("Cancel")
                .build()

            prompt.authenticate(info)
        }

        latch.await()
        return result
    }

    override fun shareText(text: String) {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, text)
        }
        activity.startActivity(Intent.createChooser(intent, null))
    }

    override fun getSafeAreaInsetsTop(): Long = 0
    override fun getSafeAreaInsetsRight(): Long = 0
    override fun getSafeAreaInsetsBottom(): Long = 0
    override fun getSafeAreaInsetsLeft(): Long = 0

    override fun openURL(url: String) {
        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url))
        activity.startActivity(intent)
    }

    override fun onAppBackground() {}
    override fun onAppForeground() {}
}
