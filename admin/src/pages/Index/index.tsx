import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { useEffect, useRef } from "react";
import type { CSSProperties } from "react";
import {
  ArrowRightOutlined,
  CheckCircleFilled,
  CloudDownloadOutlined,
  CloudServerOutlined,
  FileImageOutlined,
  GlobalOutlined,
  InboxOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { Link } from "@umijs/max";
import BrandLogo from "@/components/BrandLogo";
import ThemeToggleButton from "@/components/layout/ThemeToggleButton";
import "./index.less";

function getCapabilities() {
  return [
  {
    icon: <CloudDownloadOutlined />,
    title: tr("多来源商品采集"),
    description: tr("统一承接商品链接与采集任务，让原始商品信息有序进入草稿流程。"),
    tone: "blue",
  },
  {
    icon: <RobotOutlined />,
    title: tr("AI 商品运营"),
    description: tr("围绕标题、描述与图片生成优化建议，并保留清晰的人工复核入口。"),
    tone: "violet",
  },
  {
    icon: <ShopOutlined />,
    title: tr("多平台刊登协同"),
    description: tr("通过标准化接口扩展平台能力，集中管理刊登草稿、任务与异常处理。"),
    tone: "cyan",
  },
  {
    icon: <InboxOutlined />,
    title: tr("订单与库存协同"),
    description: tr("连接店铺、订单、规格匹配与库存同步，覆盖跨境 ERP 的关键闭环。"),
    tone: "green",
  },
  {
    icon: <CloudServerOutlined />,
    title: tr("私有化部署"),
    description: tr("基于 PostgreSQL、Redis 与 Docker Compose，适合团队自部署和二次开发。"),
    tone: "amber",
  },
  {
    icon: <SafetyCertificateOutlined />,
    title: tr("安全的人机协作"),
    description: tr("敏感配置加密保存，关键发布与客服外发动作保留人工确认。"),
    tone: "indigo",
  },
  ] as const;
}

function getWorkflow() {
  return [
  {
    number: "01",
    title: tr("采集商品"),
    description: tr("提交来源链接，创建可追踪的采集任务。"),
  },
  {
    number: "02",
    title: tr("整理草稿"),
    description: tr("统一维护商品、规格、图片与平台信息。"),
  },
  {
    number: "03",
    title: tr("AI 优化"),
    description: tr("生成标题、描述与图片处理建议。"),
  },
  {
    number: "04",
    title: tr("人工复核"),
    description: tr("确认内容、映射与发布前检查结果。"),
  },
  {
    number: "05",
    title: tr("刊登协同"),
    description: tr("创建平台任务并跟踪结果与异常。"),
  },
  ] as const;
}

const PLATFORM_LABELS = ["1688", "Douyin Shop", "Shopee", "Lazada"] as const;

function revealDelay(index: number): CSSProperties {
  return { "--landing-reveal-delay": `${index * 70}ms` } as CSSProperties;
}

export default function IndexPage() {
  useLanguage();
  const capabilities = getCapabilities();
  const workflow = getWorkflow();
  const pageRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const page = pageRef.current;
    if (!page) return undefined;

    const revealItems = Array.from(
      page.querySelectorAll<HTMLElement>("[data-landing-reveal]"),
    );
    page.classList.add("landing-page--motion-ready");

    const motionPreference = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    );
    if (
      motionPreference?.matches === true ||
      typeof window.IntersectionObserver !== "function"
    ) {
      revealItems.forEach((item) => item.classList.add("is-visible"));
      return undefined;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          entry.target.classList.add("is-visible");
          observer.unobserve(entry.target);
        });
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.12 },
    );
    revealItems.forEach((item) => observer.observe(item));

    return () => observer.disconnect();
  }, []);

  return (
    <div ref={pageRef} className="landing-page">
      <header className="landing-header">
        <div className="landing-header__inner">
          <Link to="/" className="landing-brand" aria-label={tr("TradeMind 首页")}>
            <BrandLogo height={34} />
            <span className="landing-brand__copy">
              <strong>{tr("贸灵 TradeMind")}</strong>
              <small>{tr("AI 跨境电商运营平台")}</small>
            </span>
          </Link>

          <nav className="landing-nav" aria-label={tr("首页导航")}>
            <a href="#capabilities">{tr("产品能力")}</a>
            <a href="#workflow">{tr("工作流程")}</a>
            <a href="#deployment">{tr("开源部署")}</a>
          </nav>

          <div className="landing-header__actions">
            <ThemeToggleButton className="landing-theme-toggle" />
            <Link
              to="/user/login"
              className="landing-button landing-button--text"
            >
              {tr("登录")}
            </Link>
            <Link
              to="/user/register"
              className="landing-button landing-button--primary"
            >
              {tr("免费注册")}
            </Link>
          </div>
        </div>
      </header>

      <main>
        <section className="landing-hero" aria-labelledby="landing-hero-title">
          <div
            className="landing-hero__glow landing-hero__glow--blue"
            aria-hidden="true"
          />
          <div
            className="landing-hero__glow landing-hero__glow--amber"
            aria-hidden="true"
          />
          <div className="landing-section landing-hero__inner">
            <div className="landing-hero__copy" data-landing-reveal>
              <div className="landing-eyebrow">
                <GlobalOutlined />
                <span>{tr("开源 AI 跨境电商运营平台")}</span>
              </div>
              <h1 id="landing-hero-title">
                {tr("让商品运营，")}
                <span>{tr("从采集到刊登更顺畅")}</span>
              </h1>
              <p>
                {tr("TradeMind 聚焦商品采集、草稿整理、AI 内容优化、图片处理、多平台刊登与库存协同，为跨境团队提供一套可部署、可扩展的运营工作台。")}
              </p>

              <div className="landing-hero__actions">
                <Link
                  to="/user/register"
                  className="landing-button landing-button--primary landing-button--large"
                >
                  {tr("免费注册")}
                  <ArrowRightOutlined aria-hidden="true" />
                </Link>
                <Link
                  to="/user/login"
                  className="landing-button landing-button--secondary landing-button--large"
                >
                  {tr("登录工作台")}
                </Link>
              </div>

              <ul className="landing-assurances" aria-label={tr("产品特点")}>
                <li>
                  <CheckCircleFilled /> {tr("支持私有化部署")}
                </li>
                <li>
                  <CheckCircleFilled /> {tr("能力接口可扩展")}
                </li>
                <li>
                  <CheckCircleFilled /> {tr("关键动作人工确认")}
                </li>
              </ul>
            </div>

            <div
              className="landing-preview"
              aria-label={tr("TradeMind 商品运营流程示意")}
              data-landing-reveal
              style={revealDelay(2)}
            >
              <div className="landing-preview__window">
                <div className="landing-preview__bar">
                  <span className="landing-preview__dots" aria-hidden="true">
                    <i />
                    <i />
                    <i />
                  </span>
                  <strong>{tr("商品运营工作台")}</strong>
                  <span className="landing-preview__mode">{tr("流程示意")}</span>
                </div>
                <div className="landing-preview__body">
                  <div className="landing-preview__main">
                    <div className="landing-preview__heading">
                      <div>
                        <span>{tr("商品草稿")}</span>
                        <strong>{tr("跨平台商品内容")}</strong>
                      </div>
                      <span className="landing-status">{tr("待人工复核")}</span>
                    </div>

                    <div className="landing-product-row">
                      <div className="landing-product-row__image">
                        <FileImageOutlined />
                      </div>
                      <div className="landing-product-row__content">
                        <strong>{tr("采集内容已进入草稿")}</strong>
                        <span>{tr("标题、规格、图片与平台信息集中整理")}</span>
                        <div className="landing-product-row__tags">
                          <span>{tr("标题建议")}</span>
                          <span>{tr("图片处理")}</span>
                          <span>{tr("发布检查")}</span>
                        </div>
                      </div>
                    </div>

                    <div className="landing-suggestion-list">
                      <div className="landing-suggestion-item">
                        <span className="landing-suggestion-item__icon landing-suggestion-item__icon--violet">
                          <RobotOutlined />
                        </span>
                        <div>
                          <strong>{tr("AI 内容优化")}</strong>
                          <span>{tr("建议生成后由运营人员选择应用")}</span>
                        </div>
                        <span className="landing-suggestion-item__state">
                          {tr("可复核")}
                        </span>
                      </div>
                      <div className="landing-suggestion-item">
                        <span className="landing-suggestion-item__icon landing-suggestion-item__icon--cyan">
                          <FileImageOutlined />
                        </span>
                        <div>
                          <strong>{tr("图片智能处理")}</strong>
                          <span>{tr("通过异步任务跟踪处理结果")}</span>
                        </div>
                        <span className="landing-suggestion-item__state">
                          {tr("可追踪")}
                        </span>
                      </div>
                    </div>
                  </div>

                  <aside
                    className="landing-preview__flow"
                    aria-label={tr("运营流程")}
                  >
                    <span className="landing-preview__flow-label">
                      {tr("运营流程")}
                    </span>
                    <div className="landing-flow-step landing-flow-step--active">
                      <CloudDownloadOutlined />
                      <span>{tr("商品采集")}</span>
                      <i />
                    </div>
                    <div className="landing-flow-step landing-flow-step--active">
                      <RobotOutlined />
                      <span>{tr("AI 优化")}</span>
                      <i />
                    </div>
                    <div className="landing-flow-step">
                      <SafetyCertificateOutlined />
                      <span>{tr("人工复核")}</span>
                      <i />
                    </div>
                    <div className="landing-flow-step">
                      <SyncOutlined />
                      <span>{tr("刊登协同")}</span>
                    </div>
                  </aside>
                </div>
              </div>
              <div
                className="landing-preview__platforms"
                aria-label={tr("平台扩展示例")}
              >
                {PLATFORM_LABELS.map((platform) => (
                  <span key={platform}>{platform}</span>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="landing-proof" aria-label={tr("技术特点")}>
          <div
            className="landing-section landing-proof__inner"
            data-landing-reveal
          >
            <span>{tr("开源可审计")}</span>
            <i />
            <span>PostgreSQL + Redis</span>
            <i />
            <span>Docker Compose</span>
            <i />
            <span>React + Go</span>
          </div>
        </section>

        <section
          id="capabilities"
          className="landing-content-section landing-capabilities"
          aria-labelledby="capabilities-title"
        >
          <div className="landing-section">
            <div className="landing-section-heading" data-landing-reveal>
              <span>{tr("产品能力")}</span>
              <h2 id="capabilities-title">{tr("围绕高频运营链路，减少工具切换")}</h2>
              <p>
                {tr("用清晰的任务、草稿与状态承接跨境商品运营，不把平台扩展成难以维护的重型 ERP。")}
              </p>
            </div>
            <div className="landing-capability-grid">
              {capabilities.map((item, index) => (
                <article
                  key={item.title}
                  className="landing-capability-card"
                  data-landing-reveal
                  style={revealDelay(index)}
                >
                  <span
                    className={`landing-capability-card__icon landing-capability-card__icon--${item.tone}`}
                  >
                    {item.icon}
                  </span>
                  <h3>{item.title}</h3>
                  <p>{item.description}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section
          id="workflow"
          className="landing-content-section landing-workflow"
          aria-labelledby="workflow-title"
        >
          <div className="landing-section">
            <div
              className="landing-section-heading landing-section-heading--left"
              data-landing-reveal
            >
              <span>{tr("工作流程")}</span>
              <h2 id="workflow-title">{tr("一条可追踪、可复核的商品运营主线")}</h2>
            </div>
            <ol className="landing-workflow-list">
              {workflow.map((item, index) => (
                <li
                  key={item.number}
                  data-landing-reveal
                  style={revealDelay(index)}
                >
                  <span className="landing-workflow-list__number">
                    {item.number}
                  </span>
                  <div>
                    <h3>{item.title}</h3>
                    <p>{item.description}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section
          id="deployment"
          className="landing-content-section landing-deployment"
          aria-labelledby="deployment-title"
        >
          <div className="landing-section landing-deployment__inner">
            <div className="landing-deployment__copy" data-landing-reveal>
              <span className="landing-deployment__label">
                {tr("为自部署与二次开发而设计")}
              </span>
              <h2 id="deployment-title">
                {tr("保留数据控制权，也保留持续扩展的空间")}
              </h2>
              <p>
                {tr("前后端、采集服务、PostgreSQL 与 Redis 统一维护；AI、图片、存储、平台和采集能力通过标准化接口接入。")}
              </p>
              <div className="landing-deployment__actions">
                <Link
                  to="/user/register"
                  className="landing-button landing-button--light landing-button--large"
                >
                  {tr("创建账号")} <ArrowRightOutlined aria-hidden="true" />
                </Link>
                <a
                  className="landing-button landing-button--ghost landing-button--large"
                  href="https://github.com/lien0219/trademind-ai"
                  target="_blank"
                  rel="noreferrer"
                >
                  {tr("查看开源项目")}
                </a>
              </div>
            </div>
            <div
              className="landing-deployment__stack"
              aria-label={tr("TradeMind 技术栈")}
              data-landing-reveal
              style={revealDelay(2)}
            >
              <div>
                <span>Admin</span>
                <strong>React + TypeScript</strong>
              </div>
              <div>
                <span>Backend</span>
                <strong>Go + Gin + GORM</strong>
              </div>
              <div>
                <span>Data</span>
                <strong>PostgreSQL + Redis</strong>
              </div>
              <div>
                <span>Deploy</span>
                <strong>Docker Compose</strong>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="landing-footer">
        <div className="landing-section landing-footer__inner">
          <div className="landing-brand landing-brand--footer">
            <BrandLogo height={30} />
            <span className="landing-brand__copy">
              <strong>{tr("贸灵 TradeMind")}</strong>
              <small>{tr("开源 AI 跨境电商运营平台")}</small>
            </span>
          </div>
          <span>Apache-2.0 License</span>
        </div>
      </footer>
    </div>
  );
}
