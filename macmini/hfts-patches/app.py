import os
import sys
import time
import uuid
import random
import logging
import asyncio
import urllib.request
import warnings
from typing import Optional, Union
import argparse

# 代理链支持：环境变量代理（Clash）→ 后端代理 → 目标
try:
    from proxy_chain import chain_proxy
except ImportError:
    def chain_proxy(p): return p or ""

# 抑制 requests 库 verify=False 产生的 InsecureRequestWarning 噪音
warnings.filterwarnings("ignore", message="Unverified HTTPS request")
from quart import Quart, request, jsonify
try:
    from camoufox.async_api import AsyncCamoufox
except ImportError:
    AsyncCamoufox = None
from patchright.async_api import async_playwright
from db_results import init_db, save_result, load_result, cleanup_old_results
from browser_configs import browser_config
from rich.console import Console
from rich.panel import Panel
from rich.text import Text
from rich.align import Align
from rich import box



COLORS = {
    'MAGENTA': '\033[35m',
    'BLUE': '\033[34m',
    'GREEN': '\033[32m',
    'YELLOW': '\033[33m',
    'RED': '\033[31m',
    'RESET': '\033[0m',
}


class CustomLogger(logging.Logger):
    @staticmethod
    def format_message(level, color, message):
        timestamp = time.strftime('%H:%M:%S')
        return f"[{timestamp}] [{COLORS.get(color)}{level}{COLORS.get('RESET')}] -> {message}"

    def debug(self, message, *args, **kwargs):
        super().debug(self.format_message('DEBUG', 'MAGENTA', message), *args, **kwargs)

    def info(self, message, *args, **kwargs):
        super().info(self.format_message('INFO', 'BLUE', message), *args, **kwargs)

    def success(self, message, *args, **kwargs):
        super().info(self.format_message('SUCCESS', 'GREEN', message), *args, **kwargs)

    def warning(self, message, *args, **kwargs):
        super().warning(self.format_message('WARNING', 'YELLOW', message), *args, **kwargs)

    def error(self, message, *args, **kwargs):
        super().error(self.format_message('ERROR', 'RED', message), *args, **kwargs)


logging.setLoggerClass(CustomLogger)
logger = logging.getLogger("TurnstileAPIServer")
logger.setLevel(logging.DEBUG)
handler = logging.StreamHandler(sys.stdout)
logger.addHandler(handler)


class TurnstileAPIServer:

    def __init__(self, headless: bool, useragent: Optional[str], debug: bool, browser_type: str, thread: int, proxy_support: bool, use_random_config: bool = False, browser_name: Optional[str] = None, browser_version: Optional[str] = None, default_proxy: Optional[str] = None):
        self.app = Quart(__name__)
        self.debug = debug
        self.browser_type = browser_type
        self.headless = headless
        self.thread_count = thread
        self.proxy_support = proxy_support
        self._raw_default_proxy = default_proxy or ""  # 保留原始值用于 pool 匹配比较
        self.default_proxy = chain_proxy(default_proxy) if default_proxy else None  # pool 浏览器启动时的默认代理
        self.browser_pool = asyncio.Queue()
        self.use_random_config = use_random_config
        self.browser_name = browser_name
        self.browser_version = browser_version
        self.console = Console()
        # 服务端缓存的 Turnstile API JS 内容（绕过代理压缩问题）
        self._turnstile_js_cache: Optional[str] = None

        # Initialize useragent and sec_ch_ua attributes
        self.useragent = useragent
        self.sec_ch_ua = None
        
        
        if self.browser_type in ['chromium', 'chrome', 'msedge']:
            if browser_name and browser_version:
                config = browser_config.get_browser_config(browser_name, browser_version)
                if config:
                    useragent, sec_ch_ua = config
                    self.useragent = useragent
                    self.sec_ch_ua = sec_ch_ua
            elif useragent:
                self.useragent = useragent
            else:
                browser, version, useragent, sec_ch_ua = browser_config.get_random_browser_config(self.browser_type)
                self.browser_name = browser
                self.browser_version = version
                self.useragent = useragent
                self.sec_ch_ua = sec_ch_ua
        
        self.browser_args = []
        if self.useragent:
            self.browser_args.append(f"--user-agent={self.useragent}")

        self._setup_routes()

    def display_welcome(self):
        """Displays welcome screen with logo."""
        self.console.clear()
        
        combined_text = Text()
        combined_text.append("\n📢 Channel: ", style="bold white")
        combined_text.append("https://t.me/D3_vin", style="cyan")
        combined_text.append("\n💬 Chat: ", style="bold white")
        combined_text.append("https://t.me/D3vin_chat", style="cyan")
        combined_text.append("\n📁 GitHub: ", style="bold white")
        combined_text.append("https://github.com/D3-vin", style="cyan")
        combined_text.append("\n📁 Version: ", style="bold white")
        combined_text.append("1.2b", style="green")
        combined_text.append("\n")

        info_panel = Panel(
            Align.left(combined_text),
            title="[bold blue]Turnstile Solver[/bold blue]",
            subtitle="[bold magenta]Dev by D3vin[/bold magenta]",
            box=box.ROUNDED,
            border_style="bright_blue",
            padding=(0, 1),
            width=50
        )

        self.console.print(info_panel)
        self.console.print()




    def _setup_routes(self) -> None:
        """Set up the application routes."""
        self.app.before_serving(self._startup)
        self.app.route('/turnstile', methods=['GET'])(self.process_turnstile)
        self.app.route('/result', methods=['GET'])(self.get_result)
        self.app.route('/health')(self.health)
        self.app.route('/')(self.index)
        

    async def _startup(self) -> None:
        """Initialize the browser and page pool on startup."""
        self.display_welcome()
        logger.info("Starting browser initialization")
        try:
            await init_db()
            await self._initialize_browser()
            
            # Запускаем периодическую очистку старых результатов
            asyncio.create_task(self._periodic_cleanup())
            
        except Exception as e:
            logger.error(f"Failed to initialize browser: {str(e)}")
            raise

    async def _initialize_browser(self) -> None:
        """Initialize the browser and create the page pool."""
        playwright = None
        camoufox = None

        if self.browser_type in ['chromium', 'chrome', 'msedge']:
            playwright = await async_playwright().start()
        elif self.browser_type == "camoufox":
            camoufox_kwargs = {"headless": self.headless}
            # pool 浏览器直接带代理启动（Firefox 不支持 context 级代理）
            proxy_to_use = self.default_proxy

            # 如果没有 default_proxy，尝试使用环境变量代理（HTTPS_PROXY/HTTP_PROXY）
            if not proxy_to_use:
                for var in ("HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"):
                    env_proxy = os.environ.get(var, "").strip()
                    if env_proxy:
                        proxy_to_use = env_proxy
                        logger.info(f"Using proxy from environment variable {var}: {proxy_to_use}")
                        break

            if proxy_to_use:
                proxy_config = {"server": proxy_to_use}
                if '@' in proxy_to_use:
                    scheme_part, auth_part = proxy_to_use.split('://')
                    auth, address = auth_part.split('@')
                    username, password = auth.split(':')
                    ip, port = address.split(':')
                    proxy_config = {"server": f"{scheme_part}://{ip}:{port}", "username": username, "password": password}
                camoufox_kwargs["proxy"] = proxy_config
                # 注意：不设置 os/humanize/window，使用极简参数（修复 CAPTCHA_FAIL）
                # - os="windows" 与 Linux 容器矛盾，导致指纹不一致
                # - humanize=True 增加延迟，导致 Turnstile 超时
                # - window=(1280, 720) 与 Xvfb 虚拟屏幕冲突
                camoufox_kwargs["geoip"] = True
                logger.info(f"Camoufox pool with browser-level proxy: {proxy_to_use}")
            self._camoufox_kwargs = camoufox_kwargs  # 保存参数用于浏览器崩溃后重建
            camoufox = AsyncCamoufox(**camoufox_kwargs)

        browser_configs = []
        for _ in range(self.thread_count):
            if self.browser_type in ['chromium', 'chrome', 'msedge']:
                if self.use_random_config:
                    browser, version, useragent, sec_ch_ua = browser_config.get_random_browser_config(self.browser_type)
                elif self.browser_name and self.browser_version:
                    config = browser_config.get_browser_config(self.browser_name, self.browser_version)
                    if config:
                        useragent, sec_ch_ua = config
                        browser = self.browser_name
                        version = self.browser_version
                    else:
                        browser, version, useragent, sec_ch_ua = browser_config.get_random_browser_config(self.browser_type)
                else:
                    browser = getattr(self, 'browser_name', 'custom')
                    version = getattr(self, 'browser_version', 'custom')
                    useragent = self.useragent
                    sec_ch_ua = getattr(self, 'sec_ch_ua', '')
            else:
                # Для camoufox и других браузеров используем значения по умолчанию
                browser = self.browser_type
                version = 'custom'
                useragent = self.useragent
                sec_ch_ua = getattr(self, 'sec_ch_ua', '')

            
            browser_configs.append({
                'browser_name': browser,
                'browser_version': version,
                'useragent': useragent,
                'sec_ch_ua': sec_ch_ua
            })

        for i in range(self.thread_count):
            config = browser_configs[i]

            # Docker 容器必需参数
            # --disable-features=ThirdPartyCookiePhaseout: Chrome 120+ 默认阻断第三方 Cookie，
            # Turnstile iframe (challenges.cloudflare.com) 需要跨域 Cookie，
            # 不加此 flag 会报 "Please enable cookies."
            browser_args = [
                "--no-sandbox",
                "--disable-dev-shm-usage",
                "--disable-gpu",
                "--disable-features=ThirdPartyCookiePhaseout",
            ]
            if config['useragent']:
                browser_args.append(f"--user-agent={config['useragent']}")

            browser = None
            if self.browser_type in ['chromium', 'chrome', 'msedge'] and playwright:
                browser = await playwright.chromium.launch(
                    channel=self.browser_type if self.browser_type != 'chromium' else None,
                    headless=self.headless,
                    args=browser_args
                )
            elif self.browser_type == "camoufox" and camoufox:
                browser = await camoufox.start()

            if browser:
                await self.browser_pool.put((i+1, browser, config))

            if self.debug:
                logger.info(f"Browser {i + 1} initialized successfully with {config['browser_name']} {config['browser_version']}")

        logger.info(f"Browser pool initialized with {self.browser_pool.qsize()} browsers")
        
        if self.use_random_config:
            logger.info(f"Each browser in pool received random configuration")
        elif self.browser_name and self.browser_version:
            logger.info(f"All browsers using configuration: {self.browser_name} {self.browser_version}")
        else:
            logger.info("Using custom configuration")
            
        if self.debug:
            for i, config in enumerate(browser_configs):
                logger.debug(f"Browser {i+1} config: {config['browser_name']} {config['browser_version']}")
                logger.debug(f"Browser {i+1} User-Agent: {config['useragent']}")
                logger.debug(f"Browser {i+1} Sec-CH-UA: {config['sec_ch_ua']}")

    async def _periodic_cleanup(self):
        """Periodic cleanup of old results every hour"""
        while True:
            try:
                await asyncio.sleep(3600)
                deleted_count = await cleanup_old_results(days_old=7)
                if deleted_count > 0:
                    logger.info(f"Cleaned up {deleted_count} old results")
            except Exception as e:
                logger.error(f"Error during periodic cleanup: {e}")

    async def _recreate_browser(self, index, config):
        """重建崩溃的浏览器实例，确保浏览器池容量不缩减"""
        try:
            if self.browser_type == "camoufox" and AsyncCamoufox and hasattr(self, '_camoufox_kwargs'):
                camoufox_inst = AsyncCamoufox(**self._camoufox_kwargs)
                browser = await camoufox_inst.start()
                logger.info(f"Browser {index}: 浏览器重建成功")
                return browser
            elif self.browser_type in ['chromium', 'chrome', 'msedge']:
                pw = await async_playwright().start()
                browser_args = [
                    "--no-sandbox",
                    "--disable-dev-shm-usage",
                    "--disable-gpu",
                    "--disable-features=ThirdPartyCookiePhaseout",
                ]
                if config.get('useragent'):
                    browser_args.append(f"--user-agent={config['useragent']}")
                browser = await pw.chromium.launch(
                    channel=self.browser_type if self.browser_type != 'chromium' else None,
                    headless=self.headless,
                    args=browser_args
                )
                logger.info(f"Browser {index}: 浏览器重建成功")
                return browser
        except Exception as e:
            logger.error(f"Browser {index}: 浏览器重建失败: {e}")
        return None

    async def _antishadow_inject(self, page):
        await page.add_init_script("""
          (function() {
            const originalAttachShadow = Element.prototype.attachShadow;
            Element.prototype.attachShadow = function(init) {
              const shadow = originalAttachShadow.call(this, init);
              if (init.mode === 'closed') {
                window.__lastClosedShadowRoot = shadow;
              }
              return shadow;
            };
          })();
        """)



    async def _optimized_route_handler(self, route):
        """Оптимизированный обработчик маршрутов для экономии ресурсов."""
        url = route.request.url
        resource_type = route.request.resource_type

        allowed_types = {'document', 'script', 'xhr', 'fetch'}

        allowed_domains = [
            'challenges.cloudflare.com',
            'static.cloudflareinsights.com',
            'cloudflare.com'
        ]
        
        if resource_type in allowed_types:
            await route.continue_()
        elif any(domain in url for domain in allowed_domains):
            await route.continue_() 
        else:
            await route.abort()

    async def _get_turnstile_js(self) -> Optional[str]:
        """服务端拉取 Turnstile API JS（不走代理），缓存复用。

        绕过代理丢失 Content-Encoding 导致浏览器收到压缩乱码的问题。
        返回 JS 文本，供 add_script_tag(content=...) 注入。
        """
        if self._turnstile_js_cache:
            return self._turnstile_js_cache

        api_url = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
        try:
            def _fetch() -> str:
                req = urllib.request.Request(api_url, headers={
                    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
                    'Accept': '*/*',
                    'Accept-Encoding': 'identity',  # 禁止压缩，直接拿明文
                })
                with urllib.request.urlopen(req, timeout=15) as resp:
                    return resp.read().decode('utf-8')

            loop = asyncio.get_event_loop()
            content = await loop.run_in_executor(None, _fetch)
            # 简单校验：包含 turnstile 字样才算有效
            if content and 'turnstile' in content.lower():
                self._turnstile_js_cache = content
                logger.info(f"Turnstile API JS fetched server-side: {len(content)} bytes")
                return content
            else:
                logger.warning("Turnstile API JS fetched but content looks invalid")
        except Exception as e:
            logger.warning(f"Server-side Turnstile JS fetch failed: {e}")
        return None

    async def _cf_challenge_proxy_handler(self, route, override_proxy: Optional[str] = None):
        """拦截 CF challenge 相关请求，通过 Python 代理转发。
        如果配置了代理（SOCKS5），则用相同代理转发，保证 CF challenge 请求与
        浏览器来自同一 IP，避免 Cloudflare 因 IP 不一致拒绝 Turnstile 验证。
        override_proxy：临时 Camoufox 传入实际 proxy，优先于 self.default_proxy。
        阻塞 IO 必须在 executor 里执行，不能直接调用，否则卡死 asyncio 事件循环。
        """
        req_url = route.request.url
        method = route.request.method
        skip_headers = {'host', 'connection', 'keep-alive', 'transfer-encoding',
                        'te', 'trailer', 'upgrade', 'proxy-authorization', 'proxy-authenticate'}
        headers = {k: v for k, v in route.request.headers.items()
                   if k.lower() not in skip_headers}
        post_data = route.request.post_data_buffer  # bytes | None

        try:
            proxy_url = override_proxy if override_proxy is not None else self.default_proxy  # 与浏览器相同的代理，保证 IP 一致

            def _fetch():
                import requests as req_lib
                session = req_lib.Session()
                if proxy_url:
                    session.proxies = {'http': proxy_url, 'https': proxy_url}
                resp = session.request(
                    method=method,
                    url=req_url,
                    headers=headers,
                    data=post_data,
                    timeout=30,
                    allow_redirects=True,
                    verify=False,
                )
                resp_headers = {k: v for k, v in resp.headers.items()
                                if k.lower() not in {'transfer-encoding', 'connection'}}
                return resp.status_code, resp_headers, resp.content

            loop = asyncio.get_event_loop()
            status, resp_headers, content = await loop.run_in_executor(None, _fetch)

            if self.debug:
                logger.debug(f"CF proxy [{method}] {req_url[:80]} → {status} ({len(content)}B)")
            await route.fulfill(status=status, headers=resp_headers, body=content)
        except Exception as e:
            if self.debug:
                logger.debug(f"CF proxy error [{method}] {req_url[:80]}: {e}")
            try:
                await route.fallback()
            except Exception:
                await route.abort()

    async def _block_rendering(self, page):
        """Блокировка рендеринга для экономии ресурсов"""
        await page.route("**/*", self._optimized_route_handler)

    async def _unblock_rendering(self, page):
        """Разблокировка рендеринга"""
        await page.unroute("**/*", self._optimized_route_handler)

    async def _find_turnstile_elements(self, page, index: int):
        """Умная проверка всех возможных Turnstile элементов"""
        selectors = [
            '.cf-turnstile',
            '[data-sitekey]',
            'iframe[src*="turnstile"]',
            'iframe[title*="widget"]',
            'div[id*="turnstile"]',
            'div[class*="turnstile"]'
        ]
        
        elements = []
        for selector in selectors:
            try:
                # Безопасная проверка count()
                try:
                    count = await page.locator(selector).count()
                except Exception:
                    # Если count() дает ошибку, пропускаем этот селектор
                    continue
                    
                if count > 0:
                    elements.append((selector, count))
                    if self.debug:
                        logger.debug(f"Browser {index}: Found {count} elements with selector '{selector}'")
            except Exception as e:
                if self.debug:
                    logger.debug(f"Browser {index}: Selector '{selector}' failed: {str(e)}")
                continue
        
        return elements

    async def _find_and_click_checkbox(self, page, index: int):
        """Найти и кликнуть по чекбоксу Turnstile CAPTCHA внутри iframe"""
        try:
            # Пробуем разные селекторы iframe с защитой от ошибок
            iframe_selectors = [
                'iframe[src*="challenges.cloudflare.com"]',
                'iframe[src*="turnstile"]',
                'iframe[title*="widget"]'
            ]
            
            iframe_locator = None
            for selector in iframe_selectors:
                try:
                    test_locator = page.locator(selector).first
                    # Безопасная проверка count для iframe
                    try:
                        iframe_count = await test_locator.count()
                    except Exception:
                        iframe_count = 0
                        
                    if iframe_count > 0:
                        iframe_locator = test_locator
                        if self.debug:
                            logger.debug(f"Browser {index}: Found Turnstile iframe with selector: {selector}")
                        break
                except Exception as e:
                    if self.debug:
                        logger.debug(f"Browser {index}: Iframe selector '{selector}' failed: {str(e)}")
                    continue
            
            if iframe_locator:
                try:
                    # Получаем frame из iframe
                    iframe_element = await iframe_locator.element_handle()
                    frame = await iframe_element.content_frame()
                    
                    if frame:
                        # Ищем чекбокс внутри iframe
                        checkbox_selectors = [
                            'input[type="checkbox"]',
                            '.cb-lb input[type="checkbox"]',
                            'label input[type="checkbox"]'
                        ]
                        
                        for selector in checkbox_selectors:
                            try:
                                # Полностью избегаем locator.count() в iframe - используем альтернативный подход
                                try:
                                    # Пробуем кликнуть напрямую без count проверки
                                    checkbox = frame.locator(selector).first
                                    await checkbox.click(timeout=2000)
                                    if self.debug:
                                        logger.debug(f"Browser {index}: Successfully clicked checkbox in iframe with selector '{selector}'")
                                    return True
                                except Exception as click_e:
                                    # Если прямой клик не сработал, записываем в debug но не падаем
                                    if self.debug:
                                        logger.debug(f"Browser {index}: Direct checkbox click failed for '{selector}': {str(click_e)}")
                                    continue
                            except Exception as e:
                                if self.debug:
                                    logger.debug(f"Browser {index}: Iframe checkbox selector '{selector}' failed: {str(e)}")
                                continue
                    
                        # Если нашли iframe, но не смогли кликнуть чекбокс, пробуем клик по iframe
                        try:
                            if self.debug:
                                logger.debug(f"Browser {index}: Trying to click iframe directly as fallback")
                            await iframe_locator.click(timeout=1000)
                            return True
                        except Exception as e:
                            if self.debug:
                                logger.debug(f"Browser {index}: Iframe direct click failed: {str(e)}")
                
                except Exception as e:
                    if self.debug:
                        logger.debug(f"Browser {index}: Failed to access iframe content: {str(e)}")
            
        except Exception as e:
            if self.debug:
                logger.debug(f"Browser {index}: General iframe search failed: {str(e)}")
        
        return False

    async def _try_click_strategies(self, page, index: int):
        strategies = [
            ('checkbox_click', lambda: self._find_and_click_checkbox(page, index)),
            ('direct_widget', lambda: self._safe_click(page, '.cf-turnstile', index)),
            ('iframe_click', lambda: self._safe_click(page, 'iframe[src*="turnstile"]', index)),
            ('js_click', lambda: page.evaluate("document.querySelector('.cf-turnstile')?.click()")),
            ('sitekey_attr', lambda: self._safe_click(page, '[data-sitekey]', index)),
            ('any_turnstile', lambda: self._safe_click(page, '*[class*="turnstile"]', index)),
            ('xpath_click', lambda: self._safe_click(page, "//div[@class='cf-turnstile']", index))
        ]
        
        for strategy_name, strategy_func in strategies:
            try:
                result = await strategy_func()
                if result is True or result is None:  # None означает успех для большинства стратегий
                    if self.debug:
                        logger.debug(f"Browser {index}: Click strategy '{strategy_name}' succeeded")
                    return True
            except Exception as e:
                if self.debug:
                    logger.debug(f"Browser {index}: Click strategy '{strategy_name}' failed: {str(e)}")
                continue
        
        return False

    async def _safe_click(self, page, selector: str, index: int):
        """Полностью безопасный клик с максимальной защитой от ошибок"""
        try:
            # Пробуем кликнуть напрямую без count() проверки
            locator = page.locator(selector).first
            await locator.click(timeout=1000)
            return True
        except Exception as e:
            # Логируем ошибку только в debug режиме
            if self.debug and "Can't query n-th element" not in str(e):
                logger.debug(f"Browser {index}: Safe click failed for '{selector}': {str(e)}")
            return False

    async def _inject_captcha_directly(self, page, websiteKey: str, action: str = '', cdata: str = '', index: int = 0):
        """直接向目标页面注入 Turnstile widget（比 overlay 成功率高得多）"""
        script = f"""
        // 移除旧 widget
        document.querySelectorAll('.cf-turnstile').forEach(el => el.remove());
        document.querySelectorAll('[data-sitekey]').forEach(el => el.remove());

        const captchaDiv = document.createElement('div');
        captchaDiv.className = 'cf-turnstile';
        captchaDiv.setAttribute('data-sitekey', '{websiteKey}');
        captchaDiv.setAttribute('data-callback', 'onTurnstileCallback');
        {f'captchaDiv.setAttribute("data-action", "{action}");' if action else ''}
        {f'captchaDiv.setAttribute("data-cdata", "{cdata}");' if cdata else ''}
        captchaDiv.style.position = 'fixed';
        captchaDiv.style.top = '20px';
        captchaDiv.style.left = '20px';
        captchaDiv.style.zIndex = '9999';
        captchaDiv.style.backgroundColor = 'white';
        captchaDiv.style.padding = '15px';
        captchaDiv.style.border = '2px solid #0f79af';
        captchaDiv.style.borderRadius = '8px';
        document.body.appendChild(captchaDiv);

        const setToken = (token) => {{
            let inp = document.querySelector('input[name="cf-turnstile-response"]');
            if (!inp) {{
                inp = document.createElement('input');
                inp.type = 'hidden';
                inp.name = 'cf-turnstile-response';
                document.body.appendChild(inp);
            }}
            inp.value = token;
        }};

        window.onTurnstileCallback = setToken;

        const renderWidget = () => {{
            if (window.turnstile && window.turnstile.render) {{
                try {{
                    window.turnstile.render(captchaDiv, {{
                        sitekey: '{websiteKey}',
                        {f'action: "{action}",' if action else ''}
                        {f'cdata: "{cdata}",' if cdata else ''}
                        callback: setToken,
                        'error-callback': function(e) {{ console.log('Turnstile error:', e); }}
                    }});
                }} catch(e) {{ console.log('render error:', e); }}
            }}
        }};

        if (window.turnstile) {{
            renderWidget();
        }} else {{
            const s = document.createElement('script');
            s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js';
            s.async = true;
            s.defer = true;
            s.onload = () => setTimeout(renderWidget, 500);
            document.head.appendChild(s);
        }}
        """
        await page.evaluate(script)
        if self.debug:
            logger.debug(f"Browser {index}: Injected Turnstile widget directly with sitekey: {websiteKey}")

    async def _solve_turnstile(self, task_id: str, url: str, sitekey: str, action: Optional[str] = None, cdata: Optional[str] = None, override_proxy: str = ""):
        """Solve the Turnstile challenge."""
        proxy = None
        temp_browser = None  # Camoufox + 代理时的临时浏览器
        temp_camoufox_inst = None  # 保留引用用于正确关闭 Playwright

        index, browser, browser_config = await self.browser_pool.get()

        try:
            if hasattr(browser, 'is_connected') and not browser.is_connected():
                logger.warning(f"Browser {index}: 浏览器已断开，正在重建...")
                new_browser = await self._recreate_browser(index, browser_config)
                if new_browser:
                    await self.browser_pool.put((index, new_browser, browser_config))
                else:
                    logger.error(f"Browser {index}: 重建失败，池容量已减少")
                await save_result(task_id, "turnstile", {"value": "CAPTCHA_FAIL", "elapsed_time": 0})
                return
        except Exception as e:
            if self.debug:
                logger.warning(f"Browser {index}: Cannot check browser state: {str(e)}")

        # 优先使用 API 传入的代理，其次读 proxies.txt
        if override_proxy:
            proxy = override_proxy
            logger.info(f"Browser {index}: Using override proxy: {proxy}")
        elif self.proxy_support:
            proxy_file_path = os.path.join(os.getcwd(), "proxies.txt")

            try:
                with open(proxy_file_path) as proxy_file:
                    proxies = [line.strip() for line in proxy_file if line.strip()]

                proxy = random.choice(proxies) if proxies else None

                if self.debug and proxy:
                    logger.debug(f"Browser {index}: Selected proxy: {proxy}")
                elif self.debug and not proxy:
                    logger.debug(f"Browser {index}: No proxies available")

            except FileNotFoundError:
                logger.warning(f"Proxy file not found: {proxy_file_path}")
                proxy = None
            except Exception as e:
                logger.error(f"Error reading proxy file: {str(e)}")
                proxy = None

        # Camoufox (Firefox 内核) 不支持 context 级代理，必须在浏览器启动时设置
        # 如果 override_proxy 和 pool 默认代理一致，直接用 pool 浏览器（长期存活，信任度高）
        # 否则创建临时 Camoufox 实例，用完即关
        # 注意：用原始代理值比较，避免 chain_proxy 转换后端口不同导致误判
        pool_has_proxy = bool(self._raw_default_proxy)
        proxy_matches_pool = pool_has_proxy and proxy and proxy.strip() == self._raw_default_proxy.strip()

        # 代理链：通过 Clash 链到后端代理，确保国内环境能连通海外代理
        if proxy:
            proxy = chain_proxy(proxy)

        if proxy_matches_pool:
            logger.info(f"Browser {index}: Proxy matches pool default, using pool browser directly (no temp browser)")

        if proxy and self.browser_type == "camoufox" and not proxy_matches_pool:
            try:
                proxy_config = {"server": proxy}
                if '@' in proxy:
                    scheme_part, auth_part = proxy.split('://')
                    auth, address = auth_part.split('@')
                    username, password = auth.split(':')
                    ip, port = address.split(':')
                    proxy_config = {"server": f"{scheme_part}://{ip}:{port}", "username": username, "password": password}
                camoufox_inst = AsyncCamoufox(
                    headless=self.headless,
                    proxy=proxy_config,
                    geoip=True,
                    # 注意：不设置 os/humanize/window，使用极简参数（修复 CAPTCHA_FAIL）
                )
                temp_browser = await camoufox_inst.start()
                temp_camoufox_inst = camoufox_inst  # 保留引用用于 __aexit__ 清理
                logger.info(f"Browser {index}: Temp Camoufox created with browser-level proxy: {proxy}")
            except Exception as e:
                logger.error(f"Browser {index}: Failed to create temp Camoufox with proxy: {e}")
                temp_browser = None

        # 选择实际使用的浏览器
        actual_browser = temp_browser if temp_browser else browser

        # 根据 proxy 创建浏览器 context
        if proxy and not temp_browser:
            if '@' in proxy:
                try:
                    scheme_part, auth_part = proxy.split('://')
                    auth, address = auth_part.split('@')
                    username, password = auth.split(':')
                    ip, port = address.split(':')
                    if self.debug:
                        logger.debug(f"Browser {index}: Creating context with proxy {scheme_part}://{ip}:{port} (auth: {username}:***)")
                    context_options = {
                        "proxy": {
                            "server": f"{scheme_part}://{ip}:{port}",
                            "username": username,
                            "password": password
                        },
                        "user_agent": browser_config['useragent']
                    }

                    if browser_config['sec_ch_ua'] and browser_config['sec_ch_ua'].strip():
                        context_options['extra_http_headers'] = {
                            'sec-ch-ua': browser_config['sec_ch_ua']
                        }

                    context = await actual_browser.new_context(**context_options)
                except ValueError:
                    raise ValueError(f"Invalid proxy format: {proxy}")
            else:
                parts = proxy.split(':')
                if len(parts) == 5:
                    proxy_scheme, proxy_ip, proxy_port, proxy_user, proxy_pass = parts
                    if self.debug:
                        logger.debug(f"Browser {index}: Creating context with proxy {proxy_scheme}://{proxy_ip}:{proxy_port} (auth: {proxy_user}:***)")
                    context_options = {
                        "proxy": {
                            "server": f"{proxy_scheme}://{proxy_ip}:{proxy_port}",
                            "username": proxy_user,
                            "password": proxy_pass
                        },
                        "user_agent": browser_config['useragent']
                    }

                    if browser_config['sec_ch_ua'] and browser_config['sec_ch_ua'].strip():
                        context_options['extra_http_headers'] = {
                            'sec-ch-ua': browser_config['sec_ch_ua']
                        }

                    context = await actual_browser.new_context(**context_options)
                elif len(parts) == 3:
                    if self.debug:
                        logger.debug(f"Browser {index}: Creating context with proxy {proxy}")
                    context_options = {
                        "proxy": {"server": f"{proxy}"},
                        "user_agent": browser_config['useragent']
                    }

                    if browser_config['sec_ch_ua'] and browser_config['sec_ch_ua'].strip():
                        context_options['extra_http_headers'] = {
                            'sec-ch-ua': browser_config['sec_ch_ua']
                        }

                    context = await actual_browser.new_context(**context_options)
                elif '://' in proxy:
                    # 处理 socks5://ip:port 或 http://ip:port 格式
                    if self.debug:
                        logger.debug(f"Browser {index}: Creating context with proxy {proxy}")
                    context_options = {
                        "proxy": {"server": proxy},
                        "user_agent": browser_config['useragent']
                    }

                    if browser_config['sec_ch_ua'] and browser_config['sec_ch_ua'].strip():
                        context_options['extra_http_headers'] = {
                            'sec-ch-ua': browser_config['sec_ch_ua']
                        }

                    context = await actual_browser.new_context(**context_options)
                else:
                    raise ValueError(f"Invalid proxy format: {proxy}")
        else:
            # temp_browser 存在时走这里：代理已在浏览器级别设置，context 不需要再设代理
            # 无代理时也走这里
            if self.debug:
                if temp_browser:
                    logger.debug(f"Browser {index}: Creating context on temp Camoufox (proxy at browser level)")
                else:
                    logger.debug(f"Browser {index}: Creating context without proxy")
            context_options = {"user_agent": browser_config['useragent']}

            if browser_config['sec_ch_ua'] and browser_config['sec_ch_ua'].strip():
                context_options['extra_http_headers'] = {
                    'sec-ch-ua': browser_config['sec_ch_ua']
                }

            # 注意：不使用 bypass_csp，因为它会干扰 Turnstile iframe 通信（修复 CAPTCHA_FAIL）
            # # 原注释：
            # # Turnstile managed mode 动态创建 iframe（src=challenges.cloudflare.com）
            # # 可能被页面 CSP frame-src 拦截，导致 src 始终为空、callback 不触发
            # context_options['bypass_csp'] = True

            context = await actual_browser.new_context(**context_options)

        page = await context.new_page()
        
        await self._antishadow_inject(page)

        # 注意：强制设为 False，走 inject 路径而非 natural solve（修复 CAPTCHA_FAIL）
        # natural solve 模式在数据中心 IP 上不会自动通过，会导致超时
        is_camoufox = False

        # Chrome/Camoufox 均不拦截资源：
        # 资源拦截（CSS/image/font）会让 Cloudflare 检测到页面渲染异常，
        # 导致 managed mode Turnstile iframe src 始终为空，callback 不触发。
        # 原始版本从未启用 _block_rendering，此处移除以对齐原始行为。

        if is_camoufox:
            # Camoufox (Firefox): 只隐藏 webdriver 标志
            # 不注入 window.chrome — Firefox 没有此 API，注入反而暴露指纹不一致
            await page.add_init_script("""
Object.defineProperty(navigator, 'webdriver', {
    get: () => undefined,
});
""")
        else:
            # Chromium: 隐藏 webdriver + 补全 window.chrome
            await page.add_init_script("""
Object.defineProperty(navigator, 'webdriver', {
    get: () => undefined,
});

window.chrome = {
    runtime: {},
    loadTimes: function() {},
    csi: function() {},
};
""")
        
        if self.browser_type in ['chromium', 'chrome', 'msedge']:
            await page.set_viewport_size({"width": 500, "height": 100})
            if self.debug:
                logger.debug(f"Browser {index}: Set viewport size to 500x240")

        start_time = time.time()

        try:
            if self.debug:
                logger.debug(f"Browser {index}: Starting Turnstile solve for URL: {url} with Sitekey: {sitekey} | Action: {action} | Cdata: {cdata} | Proxy: {proxy}")

            if is_camoufox:
                # === Camoufox 自然求解模式 ===
                # 不拦截资源，让页面完整加载（含 Turnstile API 及其子资源）
                # networkidle 确保 Turnstile API 完全初始化
                if self.debug:
                    logger.debug(f"Browser {index}: [Camoufox] Natural solve mode — full page load, no resource blocking")

                # 监听请求失败（调试用）
                if self.debug:
                    def _on_request_failed(req):
                        logger.debug(f"Browser {index}: Request FAILED: {req.url[:100]} - {req.failure}")
                    page.on("requestfailed", _on_request_failed)

                # 代理拦截 CF challenge 相关域，绕过 SOCKS5 对这些路径的封锁，保证 IP 一致性
                # 仅在有代理时才安装：无代理时浏览器直连 CF，Python requests 的 TLS 指纹
                # 与 Camoufox 不同，会被 Cloudflare 检测到不一致而拒绝 challenge
                _actual_proxy = proxy if proxy else self.default_proxy
                if _actual_proxy:
                    async def _cf_proxy_handler_with_actual_proxy(route, _p=_actual_proxy):
                        await self._cf_challenge_proxy_handler(route, override_proxy=_p)
                    await page.route("*://challenges.cloudflare.com/**", _cf_proxy_handler_with_actual_proxy)
                    await page.route("*://accounts.x.ai/cdn-cgi/challenge-platform/**", _cf_proxy_handler_with_actual_proxy)
                    if self.debug:
                        logger.debug(f"Browser {index}: [Camoufox] CF challenge proxy routes installed (proxy={_actual_proxy})")
                else:
                    if self.debug:
                        logger.debug(f"Browser {index}: [Camoufox] No proxy configured, letting browser handle CF challenges natively")

                # 中止 csp-reporting 请求：它们持续 NS_ERROR_ABORT 重试，会阻止 networkidle 触发
                async def _abort_csp_reporting(route):
                    await route.abort()
                await page.route("*://csp-reporting.cloudflare.com/**", _abort_csp_reporting)

                # 使用 domcontentloaded 而非 networkidle，避免 csp-reporting 重试无限延迟
                await page.goto(url, wait_until='domcontentloaded', timeout=60000)
                # 等待页面 JS 初始化（Turnstile widget 渲染需要时间）
                await asyncio.sleep(5)

                if self.debug:
                    logger.debug(f"Browser {index}: [Camoufox] Page loaded (domcontentloaded+5s), checking for existing Turnstile...")

                # Phase 1: 检查页面自带的 Turnstile 是否已生成 token（自动求解 / managed 模式低风险直通）
                await asyncio.sleep(3)
                existing_token = await page.evaluate("""() => {
                    // 检查 hidden input
                    const inputs = document.querySelectorAll('input[name="cf-turnstile-response"]');
                    for (const inp of inputs) {
                        if (inp.value && inp.value.length > 10) return inp.value;
                    }
                    // 检查 widget data-response 属性
                    const widgets = document.querySelectorAll('.cf-turnstile[data-response]');
                    for (const w of widgets) {
                        const resp = w.getAttribute('data-response');
                        if (resp && resp.length > 10) return resp;
                    }
                    return null;
                }""")

                if existing_token:
                    elapsed_time = round(time.time() - start_time, 3)
                    logger.success(f"Browser {index}: [Camoufox] Got token from page's own Turnstile — {COLORS.get('MAGENTA')}{existing_token[:10]}{COLORS.get('RESET')} in {COLORS.get('GREEN')}{elapsed_time}{COLORS.get('RESET')}s")
                    await save_result(task_id, "turnstile", {"value": existing_token, "elapsed_time": elapsed_time})
                    return

                # Phase 2: 页面 Turnstile 未自动完成，尝试点击 checkbox
                if self.debug:
                    logger.debug(f"Browser {index}: [Camoufox] No auto-token, trying to click Turnstile checkbox...")
                clicked = await self._find_and_click_checkbox(page, index)
                if clicked:
                    # 等待 token 生成
                    await asyncio.sleep(3)
                    existing_token = await page.evaluate("""() => {
                        const inputs = document.querySelectorAll('input[name="cf-turnstile-response"]');
                        for (const inp of inputs) {
                            if (inp.value && inp.value.length > 10) return inp.value;
                        }
                        return null;
                    }""")
                    if existing_token:
                        elapsed_time = round(time.time() - start_time, 3)
                        logger.success(f"Browser {index}: [Camoufox] Got token after click — {COLORS.get('MAGENTA')}{existing_token[:10]}{COLORS.get('RESET')} in {COLORS.get('GREEN')}{elapsed_time}{COLORS.get('RESET')}s")
                        await save_result(task_id, "turnstile", {"value": existing_token, "elapsed_time": elapsed_time})
                        return

                # Phase 3: 页面自带的 Turnstile 未成功，主动加载 Turnstile API 并渲染
                has_api = await page.evaluate("typeof window.turnstile !== 'undefined'")
                if self.debug:
                    current_url = page.url
                    logger.debug(f"Browser {index}: [Camoufox] Current URL: {current_url}")
                    logger.debug(f"Browser {index}: [Camoufox] Turnstile API available: {has_api}")

                if not has_api:
                    # 诊断：从页面内 fetch Turnstile API URL，看代理返回什么
                    if self.debug:
                        try:
                            fetch_diag = await page.evaluate("""async () => {
                                try {
                                    const r = await fetch('https://challenges.cloudflare.com/turnstile/v0/api.js');
                                    const t = await r.text();
                                    return {status: r.status, type: r.headers.get('content-type'), len: t.length, start: t.substring(0, 200)};
                                } catch(e) { return {error: e.message}; }
                            }""")
                            logger.debug(f"Browser {index}: [Camoufox] Turnstile API fetch diag: {fetch_diag}")
                        except Exception as e:
                            logger.debug(f"Browser {index}: [Camoufox] Fetch diag failed: {e}")

                        # 检查页面自带的 Turnstile script 标签
                        page_scripts = await page.evaluate("""() => {
                            const scripts = [...document.querySelectorAll('script[src]')];
                            return scripts.map(s => s.src).filter(u => u.includes('turnstile') || u.includes('challenges.cloudflare'));
                        }""")
                        logger.debug(f"Browser {index}: [Camoufox] Page Turnstile scripts: {page_scripts}")

                    # 服务端拉取 Turnstile JS 并以内容方式注入，绕过代理压缩问题
                    if self.debug:
                        logger.debug(f"Browser {index}: [Camoufox] Loading Turnstile API via server-side fetch + content injection...")
                    try:
                        js_content = await self._get_turnstile_js()
                        if js_content:
                            await page.add_script_tag(content=js_content)
                            if self.debug:
                                logger.debug(f"Browser {index}: [Camoufox] Injected Turnstile JS as content ({len(js_content)} bytes)")
                        else:
                            # 服务端拉取失败时降级用 URL 方式
                            await page.add_script_tag(url='https://challenges.cloudflare.com/turnstile/v0/api.js')
                            if self.debug:
                                logger.debug(f"Browser {index}: [Camoufox] Fallback: add_script_tag with URL")
                        # 轮询等待 API 初始化（最多 10 秒）
                        for _w in range(10):
                            await asyncio.sleep(1)
                            has_api = await page.evaluate("typeof window.turnstile !== 'undefined'")
                            if has_api:
                                break
                        if self.debug:
                            logger.debug(f"Browser {index}: [Camoufox] After injection: Turnstile API available: {has_api}")
                    except Exception as e:
                        if self.debug:
                            logger.debug(f"Browser {index}: [Camoufox] Script injection failed: {e}")

                if has_api:
                    # Turnstile API 已加载，创建固定位置可见容器（managed 模式要求 widget 在视口内）
                    if self.debug:
                        logger.debug(f"Browser {index}: [Camoufox] Calling turnstile.render() with sitekey: {sitekey}")
                    await page.evaluate(f"""() => {{
                        // 创建专用渲染容器，position:fixed 确保始终在视口内可见
                        let container = document.getElementById('_ts_solver_container');
                        if (!container) {{
                            container = document.createElement('div');
                            container.id = '_ts_solver_container';
                            container.style.cssText = 'position:fixed;top:0;left:0;z-index:999999;width:300px;height:70px;background:#fff;';
                            document.body.appendChild(container);
                        }}
                        // 确保 hidden input 存在（用于读取 token）
                        let inp = document.getElementById('_ts_solver_input');
                        if (!inp) {{
                            inp = document.createElement('input');
                            inp.type = 'hidden';
                            inp.id = '_ts_solver_input';
                            inp.name = 'cf-turnstile-response';
                            document.body.appendChild(inp);
                        }}
                        const setToken = (token) => {{ inp.value = token; }};
                        window._tsSolverToken = '';
                        window._tsSolverCallback = setToken;
                        try {{
                            window.turnstile.render(container, {{
                                sitekey: '{sitekey}',
                                {'action: "' + action + '",' if action else ''}
                                {'cdata: "' + cdata + '",' if cdata else ''}
                                callback: (token) => {{
                                    window._tsSolverToken = token;
                                    setToken(token);
                                }}
                            }});
                        }} catch(e) {{ console.log('turnstile.render error:', e); }}
                    }}""")
                    await asyncio.sleep(3)
                else:
                    # add_script_tag 也失败了，用旧方法 fallback
                    if self.debug:
                        logger.debug(f"Browser {index}: [Camoufox] Turnstile API still unavailable, using _inject_captcha_directly fallback")
                    await self._inject_captcha_directly(page, sitekey, action or '', cdata or '', index)
                    await asyncio.sleep(3)

            else:
                # === Chromium 模式 ===
                if self.debug:
                    logger.debug(f"Browser {index}: Loading real website directly: {url}")

                await page.goto(url, wait_until='domcontentloaded', timeout=30000)
                await self._inject_captcha_directly(page, sitekey, action or '', cdata or '', index)
                await asyncio.sleep(2)

            locator = page.locator('input[name="cf-turnstile-response"]')
            max_attempts = 30
            
            for attempt in range(max_attempts):
                try:
                    # Безопасная проверка количества элементов с токеном
                    try:
                        count = await locator.count()
                    except Exception as e:
                        if self.debug:
                            logger.debug(f"Browser {index}: Locator count failed on attempt {attempt + 1}: {str(e)}")
                        count = 0
                    
                    if count == 0:
                        if self.debug:
                            logger.debug(f"Browser {index}: No token elements found on attempt {attempt + 1}")
                            # 每 5 次尝试诊断一次 widget 状态
                            if attempt % 5 == 0:
                                try:
                                    diag = await page.evaluate("""() => {
                                        const d = {};
                                        d.hasTurnstile = typeof window.turnstile !== 'undefined';
                                        d.cfDivs = document.querySelectorAll('.cf-turnstile').length;
                                        d.iframes = document.querySelectorAll('iframe[src*="challenges.cloudflare"]').length;
                                        d.inputs = document.querySelectorAll('input[name="cf-turnstile-response"]').length;
                                        d.allIframes = document.querySelectorAll('iframe').length;
                                        // 检查 widget 内容
                                        const cfDiv = document.querySelector('.cf-turnstile');
                                        if (cfDiv) {
                                            d.cfDivHTML = cfDiv.innerHTML.substring(0, 200);
                                            d.cfDivChildren = cfDiv.children.length;
                                        }
                                        // 检查是否有错误信息
                                        const errEl = document.querySelector('[class*="error"]');
                                        if (errEl) d.errorText = errEl.textContent.substring(0, 100);
                                        return d;
                                    }""")
                                    logger.debug(f"Browser {index}: Widget diag: {diag}")
                                except Exception as e:
                                    logger.debug(f"Browser {index}: Diag failed: {e}")
                    elif count == 1:
                        # Если только один элемент, проверяем его токен
                        try:
                            token = await locator.input_value(timeout=500)
                            if token:
                                elapsed_time = round(time.time() - start_time, 3)
                                logger.success(f"Browser {index}: Successfully solved captcha - {COLORS.get('MAGENTA')}{token[:10]}{COLORS.get('RESET')} in {COLORS.get('GREEN')}{elapsed_time}{COLORS.get('RESET')} Seconds")
                                await save_result(task_id, "turnstile", {"value": token, "elapsed_time": elapsed_time})
                                return
                        except Exception as e:
                            if self.debug:
                                logger.debug(f"Browser {index}: Single token element check failed: {str(e)}")
                    else:
                        # Если несколько элементов, проверяем все по очереди
                        if self.debug:
                            logger.debug(f"Browser {index}: Found {count} token elements, checking all")

                        for i in range(count):
                            try:
                                element_token = await locator.nth(i).input_value(timeout=500)
                                if element_token:
                                    elapsed_time = round(time.time() - start_time, 3)
                                    logger.success(f"Browser {index}: Successfully solved captcha - {COLORS.get('MAGENTA')}{element_token[:10]}{COLORS.get('RESET')} in {COLORS.get('GREEN')}{elapsed_time}{COLORS.get('RESET')} Seconds")
                                    await save_result(task_id, "turnstile", {"value": element_token, "elapsed_time": elapsed_time})
                                    return
                            except Exception as e:
                                if self.debug:
                                    logger.debug(f"Browser {index}: Token element {i} check failed: {str(e)}")
                                continue

                        # 每5次检查 window._tsSolverToken 及 iframe/container 诊断
                        if self.debug and attempt % 5 == 0:
                            try:
                                diag = await page.evaluate("""() => {
                                    const c = document.getElementById('_ts_solver_container');
                                    const iframes = [...document.querySelectorAll('iframe')].map(f => ({
                                        src: f.src.substring(0, 120),
                                        title: f.title,
                                        w: f.offsetWidth, h: f.offsetHeight
                                    }));
                                    return {
                                        solverToken: window._tsSolverToken || '',
                                        containerHTML: c ? c.innerHTML.substring(0, 400) : 'NOT FOUND',
                                        containerChildren: c ? c.children.length : -1,
                                        iframes: iframes,
                                        frameCount: window.frames.length,
                                    };
                                }""")
                                logger.debug(f"Browser {index}: [Diag] attempt={attempt} {diag}")
                                # 如果 callback 已设置 token，直接返回
                                if diag.get('solverToken'):
                                    token = diag['solverToken']
                                    elapsed_time = round(time.time() - start_time, 3)
                                    logger.success(f"Browser {index}: [Camoufox] Got token via window._tsSolverToken — {token[:10]} in {elapsed_time}s")
                                    await save_result(task_id, "turnstile", {"value": token, "elapsed_time": elapsed_time})
                                    return
                            except Exception as diag_e:
                                logger.debug(f"Browser {index}: Diag failed: {diag_e}")
                    
                    # Клик стратегии только каждые 3 попытки и не сразу
                    if attempt > 2 and attempt % 3 == 0:
                        click_success = await self._try_click_strategies(page, index)
                        if not click_success and self.debug:
                            logger.debug(f"Browser {index}: All click strategies failed on attempt {attempt + 1}")
                    
                    # 自适应等待
                    wait_time = min(0.5 + (attempt * 0.05), 2.0)
                    await asyncio.sleep(wait_time)
                    
                    if self.debug and attempt % 5 == 0:
                        logger.debug(f"Browser {index}: Attempt {attempt + 1}/{max_attempts} - No valid token yet")
                        
                except Exception as e:
                    if self.debug:
                        logger.debug(f"Browser {index}: Attempt {attempt + 1} error: {str(e)}")
                    continue

            elapsed_time = round(time.time() - start_time, 3)
            await save_result(task_id, "turnstile", {"value": "CAPTCHA_FAIL", "elapsed_time": elapsed_time})
            if self.debug:
                logger.error(f"Browser {index}: Error solving Turnstile in {COLORS.get('RED')}{elapsed_time}{COLORS.get('RESET')} Seconds")
        except Exception as e:
            elapsed_time = round(time.time() - start_time, 3)
            await save_result(task_id, "turnstile", {"value": "CAPTCHA_FAIL", "elapsed_time": elapsed_time})
            if self.debug:
                logger.error(f"Browser {index}: Error solving Turnstile: {str(e)}")
        finally:
            if self.debug:
                logger.debug(f"Browser {index}: Closing browser context and cleaning up")

            try:
                await context.close()
                if self.debug:
                    logger.debug(f"Browser {index}: Context closed successfully")
            except Exception as e:
                if self.debug:
                    logger.warning(f"Browser {index}: Error closing context: {str(e)}")

            # 关闭临时 Camoufox 浏览器（代理专用实例，用完即弃）
            if temp_camoufox_inst:
                try:
                    await temp_camoufox_inst.__aexit__(None, None, None)
                    logger.info(f"Browser {index}: Temp Camoufox closed (browser + playwright)")
                except Exception as e:
                    logger.warning(f"Browser {index}: Error closing temp Camoufox: {str(e)}")
            elif temp_browser:
                try:
                    await temp_browser.close()
                    logger.info(f"Browser {index}: Temp browser closed")
                except Exception as e:
                    logger.warning(f"Browser {index}: Error closing temp browser: {str(e)}")

            # 池中的原始浏览器始终归还（不管是否用了 temp_browser）
            # 如果浏览器已崩溃，自动重建新实例替换，确保池容量不缩减
            try:
                if hasattr(browser, 'is_connected') and browser.is_connected():
                    await self.browser_pool.put((index, browser, browser_config))
                    if self.debug:
                        logger.debug(f"Browser {index}: Browser returned to pool")
                else:
                    logger.warning(f"Browser {index}: 浏览器已断开，正在重建...")
                    new_browser = await self._recreate_browser(index, browser_config)
                    if new_browser:
                        await self.browser_pool.put((index, new_browser, browser_config))
                    else:
                        logger.error(f"Browser {index}: 重建失败，池容量已减少!")
            except Exception as e:
                if self.debug:
                    logger.warning(f"Browser {index}: Error returning browser to pool: {str(e)}")






    async def health(self):
        """健康检查端点，供 CF Worker 探活"""
        return jsonify({
            "status": "ok",
            "threads": self.thread_count,
            "pool_available": self.browser_pool.qsize(),
        })

    async def process_turnstile(self):
        """Handle the /turnstile endpoint requests."""
        url = request.args.get('url')
        sitekey = request.args.get('sitekey')
        action = request.args.get('action')
        cdata = request.args.get('cdata')
        proxy = request.args.get('proxy', '')

        if not url or not sitekey:
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_WRONG_PAGEURL",
                "errorDescription": "Both 'url' and 'sitekey' are required"
            }), 200

        task_id = str(uuid.uuid4())
        await save_result(task_id, "turnstile", {
            "status": "CAPTCHA_NOT_READY",
            "createTime": int(time.time()),
            "url": url,
            "sitekey": sitekey,
            "action": action,
            "cdata": cdata
        })

        try:
            asyncio.create_task(self._solve_turnstile(task_id=task_id, url=url, sitekey=sitekey, action=action, cdata=cdata, override_proxy=proxy))

            if self.debug:
                logger.debug(f"Request completed with taskid {task_id}.")
            return jsonify({
                "errorId": 0,
                "taskId": task_id
            }), 200
        except Exception as e:
            logger.error(f"Unexpected error processing request: {str(e)}")
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_UNKNOWN",
                "errorDescription": str(e)
            }), 200

    async def get_result(self):
        """Return solved data"""
        task_id = request.args.get('id')

        if not task_id:
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_WRONG_CAPTCHA_ID",
                "errorDescription": "Invalid task ID/Request parameter"
            }), 200

        result = await load_result(task_id)
        if not result:
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_CAPTCHA_UNSOLVABLE",
                "errorDescription": "Task not found"
            }), 200

        if result == "CAPTCHA_NOT_READY" or (isinstance(result, dict) and result.get("status") == "CAPTCHA_NOT_READY"):
            return jsonify({"status": "processing"}), 200

        if isinstance(result, dict) and result.get("value") == "CAPTCHA_FAIL":
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_CAPTCHA_UNSOLVABLE",
                "errorDescription": "Workers could not solve the Captcha"
            }), 200

        if isinstance(result, dict) and result.get("value") and result.get("value") != "CAPTCHA_FAIL":
            return jsonify({
                "errorId": 0,
                "status": "ready",
                "solution": {
                    "token": result["value"]
                }
            }), 200
        else:
            return jsonify({
                "errorId": 1,
                "errorCode": "ERROR_CAPTCHA_UNSOLVABLE",
                "errorDescription": "Workers could not solve the Captcha"
            }), 200

    

    @staticmethod
    async def index():
        """Serve the API documentation page."""
        return """
            <!DOCTYPE html>
            <html lang="en">
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <title>Model Inference API</title>
                <style>
                body{font-family:system-ui,-apple-system,sans-serif;max-width:640px;margin:60px auto;padding:0 20px;background:#0d1117;color:#c9d1d9}
                h1{color:#58a6ff;font-size:1.4em}
                .card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin:12px 0}
                .label{color:#8b949e;font-size:0.85em}
                .value{color:#f0f6fc;font-weight:600}
                code{background:#1f2937;padding:2px 6px;border-radius:4px;font-size:0.9em}
                .status{color:#3fb950;font-weight:600}
                </style>
            </head><body>
            <h1>Model Inference API</h1>
            <div class="card">
            <div class="label">Model</div><div class="value">all-MiniLM-L6-v2</div>
            <div class="label" style="margin-top:8px">Task</div><div class="value">sentence-similarity</div>
            <div class="label" style="margin-top:8px">Parameters</div><div class="value">22M</div>
            <div class="label" style="margin-top:8px">Status</div><div class="status">Ready</div>
            </div>
            <div class="card">
            <div class="label">Endpoints</div>
            <div style="margin-top:6px"><code>POST /predict</code> &mdash; Run inference</div>
            <div style="margin-top:4px"><code>GET /health</code> &mdash; Health check</div>
            </div>
            <div style="margin-top:24px;color:#484f58;font-size:0.8em">Powered by ONNX Runtime</div>
            </body></html>
        """


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description="API Server")

    parser.add_argument('--no-headless', action='store_true', help='Run the browser with GUI (disable headless mode). By default, headless mode is enabled.')
    parser.add_argument('--useragent', type=str, help='User-Agent string (if not specified, random configuration is used)')
    parser.add_argument('--debug', action='store_true', help='Enable or disable debug mode for additional logging and troubleshooting information (default: False)')
    parser.add_argument('--browser_type', type=str, default='chromium', help='Specify the browser type for the solver. Supported options: chromium, chrome, msedge, camoufox (default: chromium)')
    parser.add_argument('--thread', type=int, default=4, help='Set the number of browser threads to use for multi-threaded mode. Increasing this will speed up execution but requires more resources (default: 1)')
    parser.add_argument('--proxy', action='store_true', help='Enable proxy support for the solver (Default: False)')
    parser.add_argument('--random', action='store_true', help='Use random User-Agent and Sec-CH-UA configuration from pool')
    parser.add_argument('--browser', type=str, help='Specify browser name to use (e.g., chrome, firefox)')
    parser.add_argument('--version', type=str, help='Specify browser version to use (e.g., 139, 141)')
    parser.add_argument('--host', type=str, default='0.0.0.0', help='Specify the IP address where the API solver runs. (Default: 127.0.0.1)')
    parser.add_argument('--port', type=str, default='5072', help='Set the port for the API solver to listen on. (Default: 5072)')
    parser.add_argument('--default-proxy', type=str, default='', help='Default proxy for Camoufox pool browsers (e.g., socks5://127.0.0.1:7890). Applied at browser launch level.')
    return parser.parse_args()


def create_app(headless: bool, useragent: str, debug: bool, browser_type: str, thread: int, proxy_support: bool, use_random_config: bool, browser_name: str, browser_version: str, default_proxy: str = '') -> Quart:
    server = TurnstileAPIServer(headless=headless, useragent=useragent, debug=debug, browser_type=browser_type, thread=thread, proxy_support=proxy_support, use_random_config=use_random_config, browser_name=browser_name, browser_version=browser_version, default_proxy=default_proxy or None)
    return server.app


if __name__ == '__main__':
    args = parse_args()
    browser_types = [
        'chromium',
        'chrome',
        'msedge',
        'camoufox',
    ]
    if args.browser_type not in browser_types:
        logger.error(f"Unknown browser type: {COLORS.get('RED')}{args.browser_type}{COLORS.get('RESET')} Available browser types: {browser_types}")
    else:
        app = create_app(
            headless=not args.no_headless,
            debug=args.debug,
            useragent=args.useragent,
            browser_type=args.browser_type,
            thread=args.thread,
            proxy_support=args.proxy,
            use_random_config=args.random,
            browser_name=args.browser,
            browser_version=args.version,
            default_proxy=args.default_proxy
        )
        app.run(host=args.host, port=int(args.port))
