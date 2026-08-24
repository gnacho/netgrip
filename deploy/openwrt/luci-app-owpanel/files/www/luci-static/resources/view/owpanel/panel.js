'use strict';
'require view';
'require ui';

/*
 * owpanel/panel: embeds the owpanel companion UI (served by the owpanel
 * package on port 8080) inside LuCI. Falls back to a plain link when the
 * browser blocks the iframe (mixed content if LuCI runs over HTTPS while
 * owpanel serves plain HTTP on the LAN).
 */

return view.extend({
	render: function() {
		var url = window.location.protocol + '//' + window.location.hostname + ':8080/';

		return E('div', { 'class': 'owpanel-embed' }, [
			E('p', { 'class': 'cbi-section-descr' }, [
				_('The owpanel companion panel runs on this router, port 8080. '),
				E('a', { href: url, target: '_blank', rel: 'noreferrer' }, _('Open it in a new tab'))
			]),
			E('iframe', {
				src: url,
				style: 'width:100%;min-height:82vh;border:0;border-radius:6px;background:#0b1118;'
			})
		]);
	}
});
