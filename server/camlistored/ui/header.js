/*
Copyright 2014 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

goog.provide('cam.Header');

goog.require('goog.Uri');

goog.require('cam.reactUtil');
goog.require('cam.SpritedImage');

cam.Header = React.createClass({
	displayName: 'Header',

	PIGGY_NATIVE_WIDTH: 88,
	PIGGY_NATIVE_HEIGHT: 62,
	PIGGY_MARGIN: {
		LEFT: 1,
		RIGHT: 4,
		TOP: -1,
		BOTTOM: 1,
	},

	SEARCH_MARGIN: {
		LEFT: 180,
		RIGHT: 145,
	},

	propTypes: {
		getCurrentSearch: React.PropTypes.func,
		setCurrentSearch: React.PropTypes.func,
		errors: React.PropTypes.arrayOf(
			React.PropTypes.shape({
				error: React.PropTypes.string.isRequired,
				onClick: React.PropTypes.func,
				url: React.PropTypes.string,
			}).isRequired
		).isRequired,
		pendingQuery: React.PropTypes.bool,
		height: React.PropTypes.number.isRequired,
		helpURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		homeURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		importersURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		mainControls: React.PropTypes.arrayOf(React.PropTypes.node),
		onNewPermanode: React.PropTypes.func,
		onImportShare: React.PropTypes.func,
		onSearch: React.PropTypes.func,
		onLogout: React.PropTypes.func,
		favoritesURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		statusURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		timer: React.PropTypes.shape({setTimeout:React.PropTypes.func.isRequired, clearTimeout:React.PropTypes.func.isRequired}).isRequired,
		width: React.PropTypes.number.isRequired,
		config: React.PropTypes.object.isRequired,
		readOnly: React.PropTypes.bool.isRequired,
	},

	focusSearch: function() {
		this.getSearchNode_().focus();
		this.getSearchNode_().select();
	},

	getInitialState: function() {
		return {
			menuVisible: false,
		};
	},

	render: function() {
		return React.DOM.div(
			{
				className: 'cam-header',
				style: {
					width: this.props.width,
				},
			},
			React.DOM.table(
				{
					className: 'cam-header-main',
				},
				React.DOM.tbody(null,
					React.DOM.tr(null,
						this.getPiggy_(),
						this.getTitle_(),
						this.getSearchbox_(),
						this.getMainControls_()
					)
				)
			),
			this.getMenuDropdown_()
		)
	},

	getPiggy_: function() {
		var props = {
			sheetWidth: 11,
			spriteWidth: this.PIGGY_NATIVE_WIDTH,
			spriteHeight: this.PIGGY_NATIVE_HEIGHT,
			style: cam.reactUtil.getVendorProps({
				position: 'absolute',
				left: this.PIGGY_MARGIN.LEFT,
				top: this.PIGGY_MARGIN.TOP,
				transform: 'scale(' + this.getPiggyScale_() + ')',
				transformOrigin: '0 0',
			}),
		};

		var image = function() {
			if (this.props.errors.length) {
				return React.createElement(cam.SpritedAnimation, cam.object.extend(props, {
					key: 'error',
					loopDelay: 10 * 1000,
					numFrames: 65,
					src: 'glitch/npc_piggy__x1_too_much_nibble_png_1354829441.png',
				}));
			} else if (this.props.pendingQuery) {
				return React.createElement(cam.SpritedAnimation, cam.object.extend(props, {
					key: 'pending',
					numFrames: 24,
					src: 'glitch/npc_piggy__x1_walk_png_1354829432.png',
				}));
			} else {
				return React.createElement(cam.SpritedImage, cam.object.extend(props, {
					key: 'ok',
					index: 5,
					src: 'glitch/npc_piggy__x1_chew_png_1354829433.png',
				}));
			}
		};

		return React.DOM.td(
			{
				className: 'cam-header-item',
				style: {
					minWidth: this.getPiggyWidth_() + this.PIGGY_MARGIN.LEFT + this.PIGGY_MARGIN.RIGHT,
				},
				onClick: this.handleClick_,
				onMouseEnter: this.handleMouseEnter_,
				onMouseLeave: this.handleMouseLeave_,
			},
			image.call(this)
		)
	},

	getTitle_: function() {
		return React.DOM.td(
			{
				className: 'cam-header-item cam-header-title',
				onClick: this.handleClick_,
				onMouseEnter: this.handleMouseEnter_,
				onMouseLeave: this.handleMouseLeave_,
			},
			React.DOM.span(null, 'Camli'),
			React.DOM.span(null, '\u25BE')
		);
	},

	getSearchbox_: function() {
		return React.DOM.td(
			{
				className: 'cam-header-item',
				style: {
					width: '100%',
				}
			},
			React.DOM.form(
				{
					onSubmit: this.handleSearchSubmit_,
				},
				React.DOM.input(
					{
						onChange: this.props.setCurrentSearch,
						placeholder: 'Search...',
						ref: 'searchbox',
						value: this.props.getCurrentSearch(),
					}
				)
			)
		)
	},

	getMainControls_: function() {
		return React.DOM.td(
			{
				className: classNames({
					'cam-header-item': true,
					'cam-header-main-controls': true,
					'cam-header-main-controls-empty': !this.props.mainControls.length,
				}),
			},
			this.props.mainControls
		);
	},

	getMenuDropdown_: function() {
		var errorItems = this.props.errors.map(function(err) {
			var children = [
				React.DOM.i({className: 'fa fa-exclamation-circle cam-header-menu-item-icon'}),
				err.error
			];
			return this.getMenuItem_(children, {link: err.url, onClick: err.onClick, class: 'cam-header-menu-item-error', hideOnRO: false});
		}, this);

		return React.DOM.div(
			{
				className: 'cam-header-menu-dropdown',
				onClick: this.handleDropdownClick_,
				onMouseEnter: this.handleMouseEnter_,
				onMouseLeave: this.handleMouseLeave_,
				style: cam.reactUtil.getVendorProps({
					transform: 'translate3d(0, ' + this.getMenuTranslate_() + '%, 0)',
				}),
			},
			this.getMenuItem_('Home', {link: this.props.homeURL, hideOnRO: false}),
			this.getMenuItem_('Upload...', {onClick: this.props.onUpload, hideOnRO: true}),

			// TODO(aa): Create a new permanode UI that delays creating the permanode until the user confirms, then change this to a link to that UI.
			// TODO(aa): Also I keep going back and forth about whether we should call this 'permanode' or 'set' in the UI. Hrm.
			this.getMenuItem_('New set', {onClick: this.props.onNewPermanode, hideOnRO: true}),
			this.getMenuItem_('Import share', {onClick: this.props.onImportShare, hideOnRO: true}),
			this.getMenuItem_('Importers', {link: this.props.importersURL, hideOnRO: true}),
			this.getMenuItem_('Server status', {link: this.props.statusURL, hideOnRO: false}),
			this.getMenuItem_('Favorites', {link: this.props.favoritesURL, hideOnRO: false}),
			this.getMenuItem_('Help', {link: this.props.helpURL, hideOnRO: false}),
			// We could use getMenuItem_, and only implement
			// the onClick part with Go, but we're instead also
			// reimplementing getMenuItem_ to demo that we can
			// create react elements in Go.
			this.getAboutDialog_(),
			this.getMenuItem_('Logout', {onClick: this.props.onLogout, hideOnRO: false}),
			errorItems
		);
	},

	getAboutDialog_: function() {
		return goreact.AboutMenuItem('About',
			// TODO(mpl): link to https://camlistore.org in
			// dialog text. But dialogs can only have text. So
			// we'll need to make our own modal later.
			'This is the web interface to a Camlistore server',
			'cam-header-menu-item',
			this.props.config);
	},

	getMenuItem_: function(text, opts) {
		if (this.props.readOnly && opts && opts.hideOnRO) {
			return null;
		}
		if (opts) {
			if (opts.onClick) {
				var opt_onClick = opts.onClick;
			}
			if (opts.link) {
				var opt_link = opts.link;
			}
			if (opts.class) {
				var opt_class = opts.class;
			}
		}
		if (!text || (!opt_onClick && !opt_link)) {
			return null;
		}

		var className = 'cam-header-menu-item';
		if (opt_class) {
			className += ' ' + opt_class;
		}

		var ctor = opt_link ? React.DOM.a : React.DOM.div;
		return ctor(
			{
				className: className,
				href: opt_link,
				onClick: opt_onClick,
			},
			text
		);
	},

	getMenuTranslate_: function() {
		if (this.state.menuVisible) {
			return 0;
		} else {
			// 110% because it has a shadow that we don't want to double-up with the shadow from the header.
			return -110;
		}
	},

	getPiggyHeight_: function() {
		return this.props.height - this.PIGGY_MARGIN.TOP - this.PIGGY_MARGIN.BOTTOM;
	},

	getPiggyWidth_: function() {
		return this.getPiggyScale_() * this.PIGGY_NATIVE_WIDTH;
	},

	getPiggyScale_: function() {
		return this.getPiggyHeight_() / this.PIGGY_NATIVE_HEIGHT;
	},

	handleClick_: function() {
		this.setState({menuVisible: !this.state.menuVisible});
	},

	handleMouseEnter_: function() {
		this.clearTimer_();
		this.setTimer_(true);
	},

	handleMouseLeave_: function() {
		this.clearTimer_();
		this.setTimer_(false);
	},

	handleDropdownClick_: function(e) {
		this.clearTimer_();
		this.setState({menuVisible:false});
	},

	setTimer_: function(show) {
		this.timerId_ = this.props.timer.setTimeout(this.handleTimer_.bind(null, show), 250);
	},

	clearTimer_: function() {
		if (this.timerId_) {
			this.props.timer.clearTimeout(this.timerId_);
		}
	},

	handleTimer_: function(show) {
		this.setState({menuVisible:show});
	},

	handleSearchSubmit_: function(e) {
		this.props.onSearch(this.getSearchNode_().value);
		e.preventDefault();
	},

	getSearchNode_: function() {
		return this.refs.searchbox;
	},
});
