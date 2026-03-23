"""
用 Camoufox 提取 accounts.x.ai 的 Turnstile token
策略：在 accounts.x.ai 页面上直接注入 Turnstile widget 获取 token
"""
import asyncio
import sys
import time

from camoufox.async_api import AsyncCamoufox


async def get_turnstile_token(timeout=45):
    print(f"[*] 启动 Camoufox (headless)...", flush=True)
    start = time.time()

    async with AsyncCamoufox(headless=True) as browser:
        page = await browser.new_page()

        print("[*] 加载 accounts.x.ai ...", flush=True)
        try:
            await page.goto("https://accounts.x.ai/sign-up", wait_until="domcontentloaded", timeout=30000)
            print(f"[+] 页面加载完成 ({time.time()-start:.1f}s)", flush=True)
        except Exception as e:
            print(f"[!] 页面加载异常: {e}", flush=True)

        # 策略 1: 点击 "Sign up with email"，让 React 组件加载 Turnstile
        print("[*] 策略1: 点击 'Sign up with email'...", flush=True)
        try:
            btn = await page.query_selector('text=Sign up with email')
            if btn:
                await btn.click()
                print("[+] 点击成功", flush=True)
                await asyncio.sleep(2)
            else:
                print("[-] 未找到 'Sign up with email' 按钮", flush=True)
        except Exception as e:
            print(f"[-] 点击失败: {e}", flush=True)

        # 截图看看当前状态
        await page.screenshot(path="/tmp/turnstile_debug2.png")
        print("[*] 截图已保存: /tmp/turnstile_debug2.png", flush=True)

        # 策略 2: 直接在页面注入 Turnstile widget
        print("[*] 策略2: 注入 Turnstile widget...", flush=True)
        await page.evaluate("""() => {
            window.__capturedToken = null;

            // 创建容器
            const container = document.createElement('div');
            container.id = 'injected-turnstile';
            container.style.position = 'fixed';
            container.style.bottom = '0';
            container.style.right = '0';
            container.style.zIndex = '99999';
            document.body.appendChild(container);

            // 加载 Turnstile JS
            const script = document.createElement('script');
            script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=__onTurnstileReady';
            script.async = true;

            window.__onTurnstileReady = function() {
                console.log('[Inject] Turnstile JS loaded, rendering widget...');
                try {
                    turnstile.render('#injected-turnstile', {
                        sitekey: '0x4AAAAAAAhr9JGVDZbrZOo0',
                        callback: function(token) {
                            console.log('[Inject] Token received!');
                            window.__capturedToken = token;
                        },
                        'error-callback': function(err) {
                            console.log('[Inject] Turnstile error: ' + err);
                            window.__turnstileError = err;
                        },
                        'expired-callback': function() {
                            console.log('[Inject] Token expired');
                        },
                        theme: 'light',
                        size: 'invisible',
                        'response-field': false,
                    });
                    console.log('[Inject] Widget rendered');
                } catch(e) {
                    console.log('[Inject] Render error: ' + e);
                    window.__turnstileError = e.toString();
                }
            };

            document.head.appendChild(script);
        }""")

        # 同时也监控页面上已有的 Turnstile（如果有的话）
        # 以及监控任何 iframe 中的 Turnstile response
        print("[*] 等待 Turnstile token...", flush=True)
        for i in range(timeout):
            # 检查注入的 token
            token = await page.evaluate("() => window.__capturedToken")
            if token:
                elapsed = time.time() - start
                print(f"[+] Token 获取成功 (注入方式)! ({elapsed:.1f}s)", flush=True)
                print(f"[+] Token 长度: {len(token)}", flush=True)
                print(f"TOKEN:{token}", flush=True)
                return token

            # 检查 hidden input
            token2 = await page.evaluate("""() => {
                const inputs = document.querySelectorAll('[name="cf-turnstile-response"]');
                for (const input of inputs) {
                    if (input.value) return input.value;
                }
                return null;
            }""")
            if token2:
                elapsed = time.time() - start
                print(f"[+] Token 获取成功 (hidden input)! ({elapsed:.1f}s)", flush=True)
                print(f"TOKEN:{token2}", flush=True)
                return token2

            # 检查错误
            error = await page.evaluate("() => window.__turnstileError")
            if error and i > 5:
                print(f"[-] Turnstile 错误: {error}", flush=True)
                break

            if i % 5 == 0 and i > 0:
                print(f"[*] 等待中... ({i}s)", flush=True)
                # 检查 Turnstile iframe
                frames = page.frames
                for f in frames:
                    if 'challenges.cloudflare.com' in f.url:
                        print(f"    发现 Turnstile iframe: {f.url[:80]}", flush=True)

            await asyncio.sleep(1)

        # 超时
        print(f"[-] Token 获取超时 ({timeout}s)", flush=True)
        await page.screenshot(path="/tmp/turnstile_debug3.png")
        print("    截图已保存: /tmp/turnstile_debug3.png", flush=True)

        # 打印 Turnstile 相关元素
        info = await page.evaluate("""() => {
            const result = {};
            result.turnstileExists = typeof window.turnstile !== 'undefined';
            result.iframes = Array.from(document.querySelectorAll('iframe')).map(f => f.src);
            result.inputs = Array.from(document.querySelectorAll('input[type=hidden]')).map(i => ({name: i.name, hasValue: !!i.value}));
            result.injectedDiv = !!document.getElementById('injected-turnstile');
            result.injectedDivHTML = document.getElementById('injected-turnstile')?.innerHTML?.substring(0, 200);
            return result;
        }""")
        print(f"    调试信息: {info}", flush=True)

        return None


if __name__ == "__main__":
    token = asyncio.run(get_turnstile_token(timeout=40))
    if token:
        sys.exit(0)
    else:
        sys.exit(1)
