// jsdom leaves focus on a control as it is disabled, where Chromium and WebKit
// drop it to the page; a test of whether a control keeps its focus through a
// run would pass there whatever the control did. So, as a browser does, focus
// leaves a control the moment it, or a fieldset around it, is disabled. jsdom
// will not blur an element it no longer counts as focusable, so focus passes
// through a stand-in that is, and is blurred from there to the page.
new MutationObserver(() => {
  const focused = document.activeElement
  if (!(focused instanceof HTMLElement) || !focused.matches(':disabled')) {
    return
  }

  const standIn = document.createElement('span')
  standIn.tabIndex = -1
  document.body.append(standIn)
  standIn.focus()
  standIn.blur()
  standIn.remove()
}).observe(document, { attributes: true, attributeFilter: ['disabled'], subtree: true })
