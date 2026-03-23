import random

# Chrome 版本池 — 保持与最新稳定版同步，Turnstile 会校验 UA 真实性
_CHROME_VERSIONS = [
    {"ver": "131.0.0.0", "brand_ver": "131"},
    {"ver": "132.0.0.0", "brand_ver": "132"},
    {"ver": "133.0.0.0", "brand_ver": "133"},
    {"ver": "134.0.0.0", "brand_ver": "134"},
]

# Sec-CH-UA 模板 — 匹配 Chromium 131+ 的真实格式
_SEC_CH_UA_TEMPLATE = (
    '"Chromium";v="{major}", "Google Chrome";v="{major}", "Not?A_Brand";v="99"'
)


class browser_config:
    @staticmethod
    def get_random_browser_config(browser_type):
        """返回: 浏览器名, 版本, User-Agent, Sec-CH-UA"""
        entry = random.choice(_CHROME_VERSIONS)
        ver = entry["ver"]
        major = entry["brand_ver"]
        ua = (
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            f"Chrome/{ver} Safari/537.36"
        )
        sec_ch_ua = _SEC_CH_UA_TEMPLATE.format(major=major)
        return "chrome", ver, ua, sec_ch_ua

    @staticmethod
    def get_browser_config(name, version):
        major = version.split(".")[0] if "." in version else version
        ua = (
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            f"Chrome/{version} Safari/537.36"
        )
        sec_ch_ua = _SEC_CH_UA_TEMPLATE.format(major=major)
        return ua, sec_ch_ua