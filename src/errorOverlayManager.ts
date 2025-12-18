import { DataQueryError } from '@grafana/data';
import { HdxErrorData } from './types';

const OVERLAY_CLASS = 'hdx-error-overlay';
const OVERLAY_CONTAINER_ATTR = 'data-hdx-overlay';

/**
 * Manages DOM injection of error overlays into Grafana panels.
 * This allows error overlays to work with any standard Grafana panel type.
 */
export class ErrorOverlayManager {
  private static instance: ErrorOverlayManager;
  private activeOverlays: Map<string, HTMLElement> = new Map();
  private styleInjected = false;

  static getInstance(): ErrorOverlayManager {
    if (!ErrorOverlayManager.instance) {
      ErrorOverlayManager.instance = new ErrorOverlayManager();
    }
    return ErrorOverlayManager.instance;
  }

  /**
   * Show error overlay for a specific query/panel
   */
  showOverlay(refId: string, errors: DataQueryError[]): void {
    // Inject styles if not already done
    this.injectStyles();

    // Small delay to ensure panel DOM is rendered
    setTimeout(() => {
      this.findAndInjectOverlay(refId, errors);
    }, 100);
  }

  /**
   * Remove error overlay for a specific query/panel
   */
  removeOverlay(refId: string): void {
    const overlay = this.activeOverlays.get(refId);
    if (overlay && overlay.parentNode) {
      overlay.parentNode.removeChild(overlay);
      this.activeOverlays.delete(refId);
    }
  }

  /**
   * Remove all active overlays
   */
  removeAllOverlays(): void {
    this.activeOverlays.forEach((overlay, refId) => {
      if (overlay.parentNode) {
        overlay.parentNode.removeChild(overlay);
      }
    });
    this.activeOverlays.clear();
  }

  private findAndInjectOverlay(refId: string, errors: DataQueryError[]): void {
    // Remove existing overlay for this refId if any
    this.removeOverlay(refId);

    // Find the panel container - Grafana panels have various structures
    // We'll try multiple strategies to find the right container
    const panelContainer = this.findPanelContainer(refId);

    if (panelContainer) {
      const overlay = this.createOverlayElement(refId, errors);

      // Ensure the container has relative positioning for absolute overlay
      const computedStyle = window.getComputedStyle(panelContainer);
      if (computedStyle.position === 'static') {
        panelContainer.style.position = 'relative';
      }

      panelContainer.appendChild(overlay);
      this.activeOverlays.set(refId, overlay);
    }
  }

  private findPanelContainer(refId: string): HTMLElement | null {
    // Strategy 1: Find panel by looking for error indicators and traverse up
    // Grafana shows errors with specific elements/classes
    const errorElements = document.querySelectorAll('[data-testid="data-testid Panel status error"]');
    for (const el of errorElements) {
      const panel = el.closest('[data-panelid]') as HTMLElement;
      if (panel && !panel.querySelector(`.${OVERLAY_CLASS}`)) {
        return panel;
      }
    }

    // Strategy 2: Find panels with "No data" or error states
    const panelContents = document.querySelectorAll('.panel-content');
    for (const content of panelContents) {
      const panel = content.closest('[data-panelid]') as HTMLElement;
      if (panel) {
        // Check if this panel has an error state (alert icon visible)
        const hasError = panel.querySelector('[aria-label*="error"]') ||
                        panel.querySelector('[data-testid*="error"]') ||
                        panel.querySelector('.panel-info-corner--error');
        if (hasError && !panel.querySelector(`.${OVERLAY_CLASS}`)) {
          return panel;
        }
      }
    }

    // Strategy 3: Find any panel that doesn't have an overlay yet and has error indicators
    const allPanels = document.querySelectorAll('[data-panelid]');
    for (const panel of allPanels) {
      const htmlPanel = panel as HTMLElement;
      if (!htmlPanel.querySelector(`.${OVERLAY_CLASS}`)) {
        // Look for error-related elements within this panel
        const errorIcon = htmlPanel.querySelector('[name="exclamation-triangle"]') ||
                         htmlPanel.querySelector('[aria-label*="Panel status error"]');
        if (errorIcon) {
          return htmlPanel;
        }
      }
    }

    return null;
  }

  private createOverlayElement(refId: string, errors: DataQueryError[]): HTMLElement {
    const overlay = document.createElement('div');
    overlay.className = OVERLAY_CLASS;
    overlay.setAttribute(OVERLAY_CONTAINER_ATTR, refId);

    const content = document.createElement('div');
    content.className = 'hdx-error-content';

    // Header with title and close button
    const header = document.createElement('div');
    header.className = 'hdx-error-header';

    const title = document.createElement('h4');
    title.className = 'hdx-error-title';
    title.textContent = `Query Error${errors.length > 1 ? 's' : ''}`;

    const closeBtn = document.createElement('button');
    closeBtn.className = 'hdx-error-close';
    closeBtn.innerHTML = '&times;';
    closeBtn.title = 'Dismiss';
    closeBtn.onclick = () => this.removeOverlay(refId);

    header.appendChild(title);
    header.appendChild(closeBtn);
    content.appendChild(header);

    // Error blocks
    errors.forEach((error, index) => {
      const errorBlock = this.createErrorBlock(error, index);
      content.appendChild(errorBlock);
    });

    // Copy all button if multiple errors
    if (errors.length > 1) {
      const copyAllBtn = document.createElement('button');
      copyAllBtn.className = 'hdx-error-btn';
      copyAllBtn.textContent = 'Copy All Errors';
      copyAllBtn.onclick = () => this.copyAllErrors(errors);
      content.appendChild(copyAllBtn);
    }

    overlay.appendChild(content);
    return overlay;
  }

  private createErrorBlock(error: DataQueryError, index: number): HTMLElement {
    const block = document.createElement('div');
    block.className = 'hdx-error-block';

    // Query label
    if (error.refId) {
      const label = document.createElement('div');
      label.className = 'hdx-error-label';
      label.textContent = `Query: ${error.refId}`;
      block.appendChild(label);
    }

    // Error message
    const message = document.createElement('div');
    message.className = 'hdx-error-message';
    message.textContent = error.message || 'Unknown error';
    block.appendChild(message);

    // SQL Query (if available)
    const errorData = error.data as HdxErrorData | undefined;
    if (errorData?.rawSql) {
      const sqlLabel = document.createElement('div');
      sqlLabel.className = 'hdx-error-label';
      sqlLabel.textContent = 'SQL Query:';
      block.appendChild(sqlLabel);

      const sqlCode = document.createElement('pre');
      sqlCode.className = 'hdx-error-sql';
      sqlCode.textContent = errorData.rawSql;
      block.appendChild(sqlCode);
    }

    // Copy button
    const btnRow = document.createElement('div');
    btnRow.className = 'hdx-error-btn-row';

    const copyBtn = document.createElement('button');
    copyBtn.className = 'hdx-error-btn';
    copyBtn.textContent = 'Copy Error & SQL';
    copyBtn.onclick = () => this.copyError(error);

    const copyStatus = document.createElement('span');
    copyStatus.className = 'hdx-error-copy-status';

    btnRow.appendChild(copyBtn);
    btnRow.appendChild(copyStatus);
    block.appendChild(btnRow);

    return block;
  }

  private copyError(error: DataQueryError): void {
    const errorData = error.data as HdxErrorData | undefined;
    const sql = errorData?.rawSql || 'N/A';
    const text = `Error: ${error.message || 'Unknown error'}\n\nSQL Query:\n${sql}`;

    navigator.clipboard.writeText(text).then(() => {
      this.showCopyFeedback();
    });
  }

  private copyAllErrors(errors: DataQueryError[]): void {
    const text = errors.map((error, idx) => {
      const errorData = error.data as HdxErrorData | undefined;
      const sql = errorData?.rawSql || 'N/A';
      return `Error ${idx + 1} (${error.refId || 'unknown'}):\n${error.message || 'Unknown error'}\n\nSQL Query:\n${sql}`;
    }).join('\n\n---\n\n');

    navigator.clipboard.writeText(text).then(() => {
      this.showCopyFeedback();
    });
  }

  private showCopyFeedback(): void {
    // Brief visual feedback could be added here
  }

  private injectStyles(): void {
    if (this.styleInjected) {
      return;
    }

    const style = document.createElement('style');
    style.id = 'hdx-error-overlay-styles';
    style.textContent = `
      .${OVERLAY_CLASS} {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(17, 18, 23, 0.92);
        z-index: 100;
        display: flex;
        align-items: flex-start;
        justify-content: center;
        padding: 16px;
        overflow: auto;
      }

      .hdx-error-content {
        max-width: 100%;
        width: 100%;
        color: #d8d9da;
        font-family: Inter, Helvetica, Arial, sans-serif;
      }

      .hdx-error-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 12px;
      }

      .hdx-error-title {
        color: #f55f5f;
        font-size: 14px;
        font-weight: 500;
        margin: 0;
      }

      .hdx-error-close {
        background: transparent;
        border: none;
        color: #8e8e8e;
        font-size: 20px;
        cursor: pointer;
        padding: 0 4px;
        line-height: 1;
      }

      .hdx-error-close:hover {
        color: #d8d9da;
      }

      .hdx-error-block {
        background-color: #1e1f24;
        border-radius: 4px;
        padding: 12px;
        margin-bottom: 12px;
        border-left: 3px solid #f55f5f;
      }

      .hdx-error-label {
        color: #8e8e8e;
        font-size: 11px;
        margin-bottom: 4px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .hdx-error-message {
        color: #f55f5f;
        font-family: 'Roboto Mono', monospace;
        font-size: 12px;
        white-space: pre-wrap;
        word-break: break-word;
        margin-bottom: 12px;
        line-height: 1.5;
      }

      .hdx-error-sql {
        color: #d8d9da;
        font-family: 'Roboto Mono', monospace;
        font-size: 11px;
        white-space: pre-wrap;
        word-break: break-word;
        background-color: #111217;
        padding: 8px;
        border-radius: 4px;
        margin: 4px 0 12px 0;
        max-height: 150px;
        overflow: auto;
      }

      .hdx-error-btn-row {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .hdx-error-btn {
        background-color: #3d71d9;
        color: white;
        border: none;
        border-radius: 4px;
        padding: 6px 12px;
        font-size: 12px;
        cursor: pointer;
        font-family: Inter, Helvetica, Arial, sans-serif;
      }

      .hdx-error-btn:hover {
        background-color: #5183e0;
      }

      .hdx-error-copy-status {
        color: #6ccf8e;
        font-size: 12px;
      }
    `;

    document.head.appendChild(style);
    this.styleInjected = true;
  }
}

// Export singleton instance
export const errorOverlayManager = ErrorOverlayManager.getInstance();
