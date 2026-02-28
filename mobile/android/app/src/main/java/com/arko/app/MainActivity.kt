package com.arko.app

import android.os.Bundle
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.fragment.app.FragmentActivity
import mobile.Mobile

class MainActivity : FragmentActivity() {

    private lateinit var webView: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        webView = findViewById(R.id.webview)

        val settings: WebSettings = webView.settings
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true

        webView.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest
            ): Boolean {
                val host = request.url.host ?: return true
                return host != "127.0.0.1"
            }
        }

        val nativeBridge = NativeBridgeImpl(this, applicationContext)

        Thread {
            Mobile.registerBridge(nativeBridge)
            val addr = Mobile.start(filesDir.absolutePath)
            runOnUiThread { webView.loadUrl(addr) }
        }.start()
    }

    override fun onPause() {
        super.onPause()
        Mobile.stop()
    }

    override fun onDestroy() {
        super.onDestroy()
        Mobile.stop()
    }
}
