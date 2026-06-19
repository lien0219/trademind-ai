from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont


ROOT = Path(__file__).resolve().parents[1]
IMG_DIR = ROOT / "docs" / "assets" / "img"
LOGO = ROOT / "admin" / "src" / "assets" / "logo.png"
SOCIAL_PREVIEW = IMG_DIR / "github-social-preview.png"

W, H = 1600, 900
BG_TOP = (246, 250, 255)
BG_BOTTOM = (233, 244, 250)
BLUE = (17, 111, 235)
TEAL = (19, 183, 191)
TEXT = (19, 33, 55)
MUTED = (88, 103, 128)
WHITE = (255, 255, 255)
BORDER = (214, 227, 242)
LEFT_PANEL_BOTTOM = 818

FONT_REG_CANDIDATES = [
    r"C:\Windows\Fonts\msyh.ttc",
    r"C:\Windows\Fonts\simhei.ttf",
    r"C:\Windows\Fonts\arial.ttf",
]
FONT_BOLD_CANDIDATES = [
    r"C:\Windows\Fonts\msyhbd.ttc",
    r"C:\Windows\Fonts\msyh.ttc",
    r"C:\Windows\Fonts\arialbd.ttf",
    r"C:\Windows\Fonts\arial.ttf",
]


def load_font(candidates: list[str], size: int) -> ImageFont.FreeTypeFont:
    for path in candidates:
        font_path = Path(path)
        if font_path.exists():
            return ImageFont.truetype(str(font_path), size=size)
    return ImageFont.load_default()


def rounded_mask(size: tuple[int, int], radius: int) -> Image.Image:
    mask = Image.new("L", size, 0)
    draw = ImageDraw.Draw(mask)
    draw.rounded_rectangle((0, 0, size[0], size[1]), radius=radius, fill=255)
    return mask


def add_shadow(
    base: Image.Image,
    box: tuple[int, int, int, int],
    *,
    radius: int = 28,
    blur: int = 28,
    offset: tuple[int, int] = (0, 18),
    color: tuple[int, int, int, int] = (34, 65, 108, 70),
) -> Image.Image:
    x0, y0, x1, y1 = box
    shadow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(shadow)
    draw.rounded_rectangle(
        (x0 + offset[0], y0 + offset[1], x1 + offset[0], y1 + offset[1]),
        radius=radius,
        fill=color,
    )
    return Image.alpha_composite(base, shadow.filter(ImageFilter.GaussianBlur(blur)))


def draw_pill(
    draw: ImageDraw.ImageDraw,
    xy: tuple[int, int],
    text: str,
    fill: tuple[int, int, int],
    *,
    text_fill: tuple[int, int, int] = WHITE,
    pad_x: int = 18,
    pad_y: int = 10,
    radius: int = 18,
    font: ImageFont.FreeTypeFont,
) -> int:
    bbox = draw.textbbox((0, 0), text, font=font)
    width = bbox[2] - bbox[0] + pad_x * 2
    height = bbox[3] - bbox[1] + pad_y * 2
    x, y = xy
    draw.rounded_rectangle((x, y, x + width, y + height), radius=radius, fill=fill)
    draw.text((x + pad_x, y + pad_y - 2), text, font=font, fill=text_fill)
    return width


def crop_cover(path: Path, size: tuple[int, int]) -> Image.Image:
    image = Image.open(path).convert("RGB")
    target_w, target_h = size
    src_w, src_h = image.size
    target_ratio = target_w / target_h
    src_ratio = src_w / src_h

    if src_ratio > target_ratio:
        new_w = int(src_h * target_ratio)
        left = (src_w - new_w) // 2
        image = image.crop((left, 0, left + new_w, src_h))
    else:
        new_h = int(src_w / target_ratio)
        top = (src_h - new_h) // 2
        image = image.crop((0, top, src_w, top + new_h))

    return image.resize(size, Image.Resampling.LANCZOS)


def wrap_text(
    draw: ImageDraw.ImageDraw,
    text: str,
    *,
    font: ImageFont.FreeTypeFont,
    max_width: int,
    lang: str,
) -> list[str]:
    if lang == "zh":
        units = list(text)
        sep = ""
    else:
        units = text.split(" ")
        sep = " "

    lines: list[str] = []
    line = ""
    for unit in units:
        candidate = line + (unit if not line else sep + unit)
        if draw.textlength(candidate, font=font) <= max_width:
            line = candidate
        else:
            if line:
                lines.append(line)
            line = unit
    if line:
        lines.append(line)
    return lines


def font_line_height(font: ImageFont.FreeTypeFont, *, extra: int = 0) -> int:
    bbox = font.getbbox("Ag")
    return bbox[3] - bbox[1] + extra


def make_canvas() -> Image.Image:
    base = Image.new("RGBA", (W, H), WHITE + (255,))
    pixels = base.load()
    for y in range(H):
        t = y / (H - 1)
        r = int(BG_TOP[0] * (1 - t) + BG_BOTTOM[0] * t)
        g = int(BG_TOP[1] * (1 - t) + BG_BOTTOM[1] * t)
        b = int(BG_TOP[2] * (1 - t) + BG_BOTTOM[2] * t)
        for x in range(W):
            pixels[x, y] = (r, g, b, 255)

    glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    draw = ImageDraw.Draw(glow)
    draw.ellipse((880, -60, 1560, 620), fill=(28, 170, 191, 70))
    draw.ellipse((1040, 180, 1660, 860), fill=(17, 111, 235, 50))
    draw.ellipse((-100, 640, 500, 1120), fill=(17, 111, 235, 18))
    base = Image.alpha_composite(base, glow.filter(ImageFilter.GaussianBlur(70)))

    shapes = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    draw = ImageDraw.Draw(shapes)
    draw.rounded_rectangle(
        (62, 74, 748, 818),
        radius=38,
        fill=(255, 255, 255, 170),
        outline=(255, 255, 255, 210),
        width=2,
    )
    draw.polygon(
        [(1110, 120), (1520, 120), (1520, 210), (1285, 260), (1080, 210)],
        fill=(255, 255, 255, 70),
    )
    draw.polygon(
        [(980, 720), (1430, 720), (1560, 860), (870, 860)],
        fill=(255, 255, 255, 90),
    )
    return Image.alpha_composite(base, shapes)


def place_card(
    base: Image.Image,
    box: tuple[int, int, int, int],
    image_path: Path,
    title: str,
    *,
    font: ImageFont.FreeTypeFont,
) -> Image.Image:
    base = add_shadow(base, box)
    x0, y0, x1, y1 = box
    card = Image.new("RGBA", (x1 - x0, y1 - y0), WHITE + (255,))
    mask = rounded_mask(card.size, 28)

    image_height = int(card.size[1] * 0.78)
    cover = crop_cover(image_path, (card.size[0], image_height)).convert("RGBA")
    card.alpha_composite(cover, (0, 0))
    card.alpha_composite(
        Image.new("RGBA", (card.size[0], image_height), (14, 28, 48, 26)),
        (0, 0),
    )

    draw = ImageDraw.Draw(card)
    draw.rounded_rectangle(
        (0, 0, card.size[0] - 1, card.size[1] - 1),
        radius=28,
        outline=BORDER,
        width=2,
    )
    draw.rectangle((0, image_height, card.size[0], card.size[1]), fill=(250, 252, 255))
    draw.text((22, image_height + 18), title, font=font, fill=TEXT)

    base.paste(card, (x0, y0), mask)
    return base


def draw_text_block(base: Image.Image, *, lang: str) -> Image.Image:
    if lang == "zh":
        brand = "贸灵 TradeMind"
        title = "让跨境商品运营\n更轻、更快、更可扩展"
        subtitle = "开源 AI 跨境电商运营平台，聚焦商品采集、商品草稿、AI 内容优化、刊登与订单库存协同。"
        pills = ["AI 商品运营", "跨平台 ERP MVP", "Self-hosted"]
        bullets = [
            "围绕高频运营链路，而不是一次性做成重型 ERP",
            "Provider 架构可扩展 AI、平台、图片、采集与存储能力",
            "适合私有化部署、团队协作与二次开发",
        ]
        candidate_sizes = [
            {"title": 58, "sub": 28, "body": 32, "small": 24, "pill": 24},
            {"title": 56, "sub": 27, "body": 32, "small": 23, "pill": 23},
        ]
    else:
        brand = "TradeMind"
        title = "Open-source AI operations\nfor cross-border commerce"
        subtitle = "Built for product collection, drafts, AI content optimization, publishing, and practical order/inventory workflows."
        pills = ["AI Product Ops", "ERP MVP", "Self-hosted"]
        bullets = [
            "Designed around daily operations, not a heavy all-in-one ERP",
            "Provider-based architecture for AI, platforms, images, storage, and collectors",
            "Ready for self-hosting, team collaboration, and secondary development",
        ]
        candidate_sizes = [
            {"title": 46, "sub": 28, "body": 32, "small": 24, "pill": 24},
            {"title": 44, "sub": 26, "body": 31, "small": 22, "pill": 23},
            {"title": 42, "sub": 25, "body": 30, "small": 21, "pill": 22},
        ]

    draw = ImageDraw.Draw(base)
    logo = Image.open(LOGO).convert("RGBA")
    logo.thumbnail((86, 86), Image.Resampling.LANCZOS)
    base.alpha_composite(logo, (104, 104))

    selected = None
    for sizes in candidate_sizes:
        font_title = load_font(FONT_BOLD_CANDIDATES, sizes["title"])
        font_sub = load_font(FONT_REG_CANDIDATES, sizes["sub"])
        font_body = load_font(FONT_REG_CANDIDATES, sizes["body"])
        font_small = load_font(FONT_REG_CANDIDATES, sizes["small"])
        font_pill = load_font(FONT_BOLD_CANDIDATES, sizes["pill"])

        title_bbox = draw.multiline_textbbox((104, 218), title, font=font_title, spacing=10)
        title_bottom = title_bbox[3]

        subtitle_lines = wrap_text(draw, subtitle, font=font_sub, max_width=596, lang=lang)[:3]
        subtitle_y = title_bottom + 28
        subtitle_step = font_line_height(font_sub, extra=10)
        subtitle_bottom = subtitle_y + subtitle_step * len(subtitle_lines)

        pill_bbox = draw.textbbox((0, 0), pills[0], font=font_pill)
        pill_height = pill_bbox[3] - pill_bbox[1] + 20
        pill_y = subtitle_bottom + 20

        bullet_y = pill_y + pill_height + 34
        bullet_step = font_line_height(font_small, extra=6)
        bullet_gap = 20 if lang == "zh" else 16
        bottom = bullet_y
        wrapped_bullets: list[list[str]] = []
        for bullet in bullets:
            lines = wrap_text(draw, bullet, font=font_small, max_width=520, lang=lang)[:2]
            wrapped_bullets.append(lines)
            bottom += bullet_step * len(lines) + bullet_gap

        if bottom <= LEFT_PANEL_BOTTOM - 16:
            selected = {
                "font_title": font_title,
                "font_sub": font_sub,
                "font_body": font_body,
                "font_small": font_small,
                "font_pill": font_pill,
                "subtitle_lines": subtitle_lines,
                "subtitle_y": subtitle_y,
                "subtitle_step": subtitle_step,
                "pill_y": pill_y,
                "bullet_y": bullet_y,
                "bullet_step": bullet_step,
                "bullet_gap": bullet_gap,
                "wrapped_bullets": wrapped_bullets,
            }
            break

    if selected is None:
        sizes = candidate_sizes[-1]
        selected = {
            "font_title": load_font(FONT_BOLD_CANDIDATES, sizes["title"]),
            "font_sub": load_font(FONT_REG_CANDIDATES, sizes["sub"]),
            "font_body": load_font(FONT_REG_CANDIDATES, sizes["body"]),
            "font_small": load_font(FONT_REG_CANDIDATES, sizes["small"]),
            "font_pill": load_font(FONT_BOLD_CANDIDATES, sizes["pill"]),
            "subtitle_lines": wrap_text(
                draw,
                subtitle,
                font=load_font(FONT_REG_CANDIDATES, sizes["sub"]),
                max_width=596,
                lang=lang,
            )[:3],
            "subtitle_y": 390 if lang == "en" else 412,
            "subtitle_step": font_line_height(load_font(FONT_REG_CANDIDATES, sizes["sub"]), extra=10),
            "pill_y": 520,
            "bullet_y": 608 if lang == "en" else 620,
            "bullet_step": font_line_height(load_font(FONT_REG_CANDIDATES, sizes["small"]), extra=6),
            "bullet_gap": 16 if lang == "en" else 20,
            "wrapped_bullets": [
                wrap_text(
                    draw,
                    bullet,
                    font=load_font(FONT_REG_CANDIDATES, sizes["small"]),
                    max_width=520,
                    lang=lang,
                )[:2]
                for bullet in bullets
            ],
        }

    draw.text((210, 114), brand, font=selected["font_body"], fill=TEXT)
    draw.text((104, 218), title, font=selected["font_title"], fill=TEXT, spacing=10)

    y = selected["subtitle_y"]
    for line in selected["subtitle_lines"]:
        draw.text((104, y), line, font=selected["font_sub"], fill=MUTED)
        y += selected["subtitle_step"]

    pill_x = 104
    pill_colors = [BLUE, (40, 153, 189), (37, 99, 235)]
    for text, fill in zip(pills, pill_colors):
        pill_x += draw_pill(draw, (pill_x, selected["pill_y"]), text, fill, font=selected["font_pill"]) + 14

    bullet_y = selected["bullet_y"]
    for bullet_lines in selected["wrapped_bullets"]:
        draw.rounded_rectangle((108, bullet_y + 12, 120, bullet_y + 24), radius=6, fill=TEAL)
        for idx, line in enumerate(bullet_lines):
            draw.text((140, bullet_y + idx * selected["bullet_step"]), line, font=selected["font_small"], fill=TEXT)
        bullet_y += selected["bullet_step"] * len(bullet_lines) + selected["bullet_gap"]

    return base


def build(lang: str, output_name: str) -> None:
    font_card = load_font(FONT_BOLD_CANDIDATES, 24)
    base = make_canvas()
    base = draw_text_block(base, lang=lang)

    labels = {
        "zh": {
            "top": "采集中心",
            "left": "采集任务",
            "right": "AI 描述生成",
            "badge": "产品预览",
        },
        "en": {
            "top": "Collection Center",
            "left": "Collection Tasks",
            "right": "AI Description",
            "badge": "Product Preview",
        },
    }[lang]

    base = place_card(base, (860, 118, 1510, 390), IMG_DIR / "2.png", labels["top"], font=font_card)
    base = place_card(base, (800, 460, 1118, 760), IMG_DIR / "3.png", labels["left"], font=font_card)
    base = place_card(base, (1160, 438, 1518, 820), IMG_DIR / "1.png", labels["right"], font=font_card)

    slim = crop_cover(IMG_DIR / "4.png", (360, 120)).convert("RGBA")
    slim_card = Image.new("RGBA", (396, 156), WHITE + (255,))
    slim_card.alpha_composite(slim, (18, 18))
    ImageDraw.Draw(slim_card).rounded_rectangle((0, 0, 395, 155), radius=24, outline=BORDER, width=2)
    slim_mask = rounded_mask(slim_card.size, 24)
    slim_box = (1090, 348, 1486, 504)
    base = add_shadow(base, slim_box, radius=24, blur=24, offset=(0, 12), color=(24, 64, 102, 55))
    base.paste(slim_card, (1090, 348), slim_mask)

    badge_font = load_font(FONT_REG_CANDIDATES, 24)
    draw_pill(
        ImageDraw.Draw(base),
        (1260, 80),
        labels["badge"],
        (16, 95, 180),
        font=badge_font,
        pad_x=16,
        pad_y=8,
        radius=16,
    )

    output_path = IMG_DIR / output_name
    output_path.parent.mkdir(parents=True, exist_ok=True)
    base.convert("RGB").save(output_path, quality=95)
    print(f"saved {output_path}")


def build_social_preview() -> None:
    source = Image.open(IMG_DIR / "readme-hero-en.png").convert("RGB")
    resized = source.resize((1280, 720), Image.Resampling.LANCZOS)
    cropped = resized.crop((0, 40, 1280, 680))
    cropped.save(SOCIAL_PREVIEW, quality=95)
    print(f"saved {SOCIAL_PREVIEW}")


if __name__ == "__main__":
    build("zh", "readme-hero-zh.png")
    build("en", "readme-hero-en.png")
    build_social_preview()
