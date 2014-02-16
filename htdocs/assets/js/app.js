var satsumaApp = angular.module('satsuma', [ 'ngRoute', 'ngUpload' ]);

satsumaApp.config(['$routeProvider', '$locationProvider', '$logProvider', function($routeProvider, $locationProvider, $logProvider) {
		$logProvider.debugEnabled(true);

		$locationProvider.html5Mode(true);

		$routeProvider.when('/contact', { templateUrl: '/assets/partials/contact.html', controller: 'StaticPageCtrl' });
		$routeProvider.when('/tos', { templateUrl: '/assets/partials/tos.html', controller: 'StaticPageCtrl' });
		$routeProvider.when('/v/:uploadid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });
		$routeProvider.when('/s/:sessionid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });
		$routeProvider.when('/settings', { templateUrl: '/assets/partials/settings.html', controller: 'SettingsCtrl' });
		$routeProvider.when('/', { templateUrl: '/assets/partials/main.html', controller: 'MainCtrl' });

		$routeProvider.otherwise({ redirectTo: '/' });
	}
]);

satsumaApp.controller('StaticPageCtrl', [ '$scope', function($scope) {
	// nothing.
}]);

satsumaApp.controller('PDFViewCtrl', [ '$scope', '$routeParams', '$http', '$location', '$log', function($scope, $routeParams, $http, $location, $log) {
	if ($routeParams.uploadid) {
		$scope.type = "viewer";
		$scope.id = $routeParams.uploadid;
	} else if ($routeParams.sessionid) {
		$scope.sessionId = $routeParams.sessionid;
		$scope.type = "session";
	};
	$log.log('PDFViewCtrl: new instance. type = ' + $scope.type);

	$scope.pageNum = 1;
	$scope.loadProgress = 0;
	$scope.fullscreen = false;
	$scope.pdfDoc = null;
	$scope.origScale = 1.0;
	$scope.scale = $scope.origScale;

	$scope.isMouseDown = false;
	$scope.lineWidth = 10;
	$scope.lineColor = '#ADFF2F';
	$scope.mouseCoords = [ ];
	$scope.oldX = $scope.oldY = 0;
	$scope.cmds = [ ];
	$scope.ended = null;

	$scope.documentProgress = function(progressData) {
		$log.log(progressData);
		$scope.loadProgress = (100 * progressData.loaded) / progressData.total;
		$scope.loadProgress = Math.round($scope.loadProgress*100)/100;
		if ($scope.loadProgress > 100) {
			$scope.loadProgress = 100;
		}
		$log.log('documentProgress: loadProgress =', $scope.loadProgress);
		$scope.$apply();
	};

	$scope.loadPDF = function(path) {
		PDFJS.getDocument(path, null, null, $scope.documentProgress).then(function(_pdfDoc) {
			$scope.scale = $scope.origScale;
			$scope.pdfDoc = _pdfDoc;
			$scope.renderPage($scope.pageNum, function(success) {
				$scope.loadProgress = 0;
				$scope.$apply();
			});
		}, function(message, exception) {
			$log.log("PDF load error: " + message);
		});
	};

	$scope.renderPage = function(num, callback) {
		$scope.pdfDoc.getPage(num).then(function(page) {
			var viewport = page.getViewport($scope.scale);
			if ($scope.fullscreen) {
				var new_scale = Math.min($scope.scale * (window.screen.height / viewport.height), $scope.scale * (window.screen.width / viewport.width));
				$log.log('scale = ' + $scope.scale + ' new scale = ' + new_scale);
				viewport = page.getViewport(new_scale);
			}

			var canvas = document.getElementById('slide_canvas');
			var ctx = canvas.getContext('2d');

			canvas.height = viewport.height;
			canvas.width = viewport.width;

			page.render({ canvasContext: ctx, viewport: viewport }).promise.then(
				function() {
					for (var i=0;i<$scope.cmds.length;i++) {
						var cmd = $scope.cmds[i];
						if (cmd.page == num && cmd.cmd != "gotoPage") {
							$scope.executeCommand(cmd);
						}
					}
					if (callback)
						callback(true);
				},
				function() {
					if (callback)
						callback(false);
				}
			);
		});
	};

	$scope.executeCommand = function(cmd) {
		switch (cmd.cmd) {
		case "drawLine":
			$scope.drawLine(cmd);
			break;
		case "gotoPage":
			$scope.pageNum = cmd.page;
			$scope.renderPage($scope.pageNum, null);
			break;
		case "clearSlide":
			$scope.cmds = _.reject($scope.cmds, function(cmd) { return cmd.page == $scope.pageNum; });
			$scope.renderPage($scope.pageNum, null);
			break;
		case "close":
			$log.log('received close command');
			$scope.ended = cmd.timestamp;
			if ($scope.ws) {
				$scope.ws.close();
				$scope.ws = null;
			}
			$scope.$apply();
			break;
		default:
			$log.log('unknown/unimplemented command ' + cmd.cmd);
		}
	};

	$scope.drawLine = function(cmd) {
		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		var xFactor = canvas.width / cmd.canvasWidth;
		var yFactor = canvas.height / cmd.canvasHeight;

		ctx.beginPath();

		ctx.lineWidth = cmd.width * xFactor;
		ctx.strokeStyle = cmd.color;
		ctx.globalAlpha = 0.5;

		for (var i=0;i<cmd.coords.length;i+=2) {
			ctx.lineTo(cmd.coords[i] * xFactor, cmd.coords[i+1] * yFactor);
		}
		ctx.stroke();
		ctx.closePath();
	};

	$scope.exit = function() {
		if ($scope.ws) {
			$scope.ws.close();
			$scope.ws = null;
		}
		$location.path('/');
	};

	$scope.clearSlide = function() {
		$scope.sendCmd({"cmd": "clearSlide", "page": $scope.pageNum});
		$scope.cmds = _.reject($scope.cmds, function(cmd) { return cmd.page == $scope.pageNum; });
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.zoomIn = function() {
		$scope.scale *= 1.2;
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.zoomOut = function() {
		$scope.scale /= 1.2;
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.gotoFullscreen = function() {
		$('#canvas_wrapper').unbind('webkitfullscreenchange mozfullscreenchange fullscreenchange');
		$('#canvas_wrapper').bind('webkitfullscreenchange mozfullscreenchange fullscreenchange', function() {
			$scope.fullscreen = !$scope.fullscreen;
			$log.log('fullscreen change: fullscreen = ' + $scope.fullscreen);
			if ($scope.fullscreen) {
				$(window).keydown($scope.handleKey);
			} else {
				$(window).unbind('keydown', $scope.handleKey);
			}
			$scope.renderPage($scope.pageNum, null);
		});

		// request the fullscreen change.
		var canvas_wrapper = document.getElementById('canvas_wrapper');
		if (canvas_wrapper.requestFullscreen) {
			canvas_wrapper.requestFullscreen();
		} else if (canvas_wrapper.mozRequestFullScreen) {
			canvas_wrapper.mozRequestFullScreen();
		} else if (canvas_wrapper.webkitRequestFullscreen) {
			canvas_wrapper.webkitRequestFullScreen();
		} else {
			$log.log('no requestFullscreen function found!');
		}
	};

	$scope.handleKey = function(event) {
		switch (event.which) {
		case 37: // left
		case 38: // up
			$scope.gotoPrev();
			break;
		case 32: // space
		case 39: // right
		case 40: // down
			$scope.gotoNext();
			break;
		}
	};

	$scope.gotoPrev = function() {
		if ($scope.pageNum > 1) {
			$scope.pageNum--;
			$scope.sendCmd({"cmd": "gotoPage", "page": $scope.pageNum});
			$scope.renderPage($scope.pageNum, null);
		}
	};

	$scope.gotoNext = function() {
		if ($scope.pageNum < $scope.pdfDoc.numPages) {
			$scope.pageNum++;
			$scope.sendCmd({"cmd": "gotoPage", "page": $scope.pageNum});
			$scope.renderPage($scope.pageNum, null);
		}
	};

	$scope.gotoPage = function(page) {
		$scope.pageNum = page;
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.sendCmd = function(data) {
		var jsonData = JSON.stringify(data);
		if ($scope.wsSend) {
			$log.log('sendCmd: sending data: ' + jsonData);
			$scope.ws.send(jsonData);
			$scope.cmds.push(data);
		}
	};

	$scope.openWebSocketMaster = function() {
		$log.log('WebSocket: onopen for master called');
		$scope.ws.send(JSON.stringify({"session_id": $scope.sessionId}));
		$scope.wsSend = true;
	};

	$scope.openWebSocketSlave = function() {
		$log.log('WebSocket: onopen for slave called');
		$scope.ws.send(JSON.stringify({"session_id": $scope.sessionId}));
	};

	$scope.onMessageSlave = function(evt) {
		$log.log('onMessageSlave: received message from server');
		var data = JSON.parse(evt.data);
		$scope.executeCommand(data);
		$scope.cmds.push(data);
	};

	$scope.reconnectWebsocket = function(evt) {
		if ($scope.ws) {
			$log.log('reconnecting WebSocket');
			var newWs = new WebSocket($scope.wsURL);
			newWs.onclose = $scope.ws.onclose;
			newWs.onopen = $scope.ws.onopen;
			newWs.onmessage = $scope.ws.onmessage;
			newWs.onerror = $scope.ws.onerror;
			$scope.ws = newWs;
		}
	};

	$scope.logWebsocketError = function(evt) {
		$log.log('websocket error: ' + evt.data);
	};

	$scope.bindCanvas = function() {
		$('#slide_canvas').mousedown($scope.mouseDown);
		$('#slide_canvas').mousemove($scope.mouseMove);
		$('#slide_canvas').mouseup($scope.mouseUp);

		var canvas = document.getElementById('slide_canvas');
		canvas.ontouchstart = $scope.touchStart;
		canvas.ontouchmove = $scope.touchMove;
		canvas.ontouchend = $scope.touchEnd;
		//$('#slide_canvas').bind('touchstart', $scope.touchStart);
		//$('#slide_canvas').bind('touchmove', $scope.touchMove);
		//$('#slide_canvas').bind('touchend', $scope.touchEnd);
	};

	$scope.relCoords = function(canvas, evt) {
		var rect = canvas.getBoundingClientRect();
		return { x: evt.clientX - rect.left, y: evt.clientY - rect.top };
	};

	$scope.mouseDown = function(e) {
		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		$scope.isMouseDown = true;

		ctx.lineWidth = $scope.lineWidth;
		ctx.strokeStyle = $scope.lineColor;
		ctx.globalAlpha = 0.5;

		$scope.mouseCoords = [ ];

		var coords = $scope.relCoords(canvas, e);

		$scope.mouseCoords.push(coords.x);
		$scope.mouseCoords.push(coords.y);

		ctx.beginPath();
		ctx.moveTo(coords.x, coords.y);
		ctx.stroke();
		ctx.closePath();
		$scope.oldX = coords.x;
		$scope.oldY = coords.y;
	};

	$scope.mouseMove = function(e) {
		if (!$scope.isMouseDown)
			return;

		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');


		ctx.lineWidth = $scope.lineWidth;
		ctx.strokeStyle = $scope.lineColor;
		ctx.globalAlpha = 0.5;

		var coords = $scope.relCoords(canvas, e);

		if (Math.abs(coords.x - $scope.oldX) > $scope.lineWidth || Math.abs(coords.y - $scope.oldY) > $scope.lineWidth) {
			ctx.beginPath();
			ctx.moveTo($scope.oldX, $scope.oldY);
			ctx.lineTo(coords.x, coords.y);
			ctx.stroke();
			ctx.closePath();

			$scope.mouseCoords.push(coords.x);
			$scope.mouseCoords.push(coords.y);

			$scope.oldX = coords.x;
			$scope.oldY = coords.y;
		}
	};

	$scope.mouseUp = function(e) {
		if (!$scope.isMouseDown)
			return;

		var canvas = document.getElementById('slide_canvas');

		$scope.oldX = $scope.oldY = 0;
		$scope.sendCmd({"cmd": "drawLine", "coords": $scope.mouseCoords, "color": $scope.lineColor, "width": $scope.lineWidth, "page": $scope.pageNum, "canvasWidth": canvas.width, "canvasHeight": canvas.height});
		$scope.mouseCoords = [ ];
		$scope.isMouseDown = false;
	};

	$scope.touchStart = function(e) {
		e.preventDefault();

		if (e.touches.length > 1) {
			return;
		}

		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		$scope.isMouseDown = true;

		ctx.lineWidth = $scope.lineWidth;
		ctx.strokeStyle = $scope.lineColor;
		ctx.globalAlpha = 0.5;

		$scope.mouseCoords = [ ];

		var coords = $scope.relCoords(canvas, e.touches[0]);

		$scope.mouseCoords.push(coords.x);
		$scope.mouseCoords.push(coords.y);

		ctx.beginPath();
		ctx.moveTo(coords.x, coords.y);
		ctx.stroke();
		ctx.closePath();
		$scope.oldX = coords.x;
		$scope.oldY = coords.y;
	};

	$scope.touchMove = function(e) {
		e.preventDefault();

		if (!$scope.isMouseDown)
			return;

		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		ctx.lineWidth = $scope.lineWidth;
		ctx.strokeStyle = $scope.lineColor;
		ctx.globalAlpha = 0.5;

		var coords = $scope.relCoords(canvas, e.touches[0]);

		if (Math.abs(coords.x - $scope.oldX) > $scope.lineWidth || Math.abs(coords.y - $scope.oldY) > $scope.lineWidth) {
			ctx.beginPath();
			ctx.moveTo($scope.oldX, $scope.oldY);
			ctx.lineTo(coords.x, coords.y);
			ctx.stroke();
			ctx.closePath();

			$scope.mouseCoords.push(coords.x);
			$scope.mouseCoords.push(coords.y);

			$scope.oldX = coords.x;
			$scope.oldY = coords.y;
		}

	};

	$scope.touchEnd = function(e) {
		e.preventDefault();

		if (!$scope.isMouseDown)
			return;

		var canvas = document.getElementById('slide_canvas');

		$scope.oldX = $scope.oldY = 0;
		$scope.sendCmd({"cmd": "drawLine", "coords": $scope.mouseCoords, "color": $scope.lineColor, "width": $scope.lineWidth, "page": $scope.pageNum, "canvasWidth": canvas.width, "canvasHeight": canvas.height});
		$scope.mouseCoords = [ ];
		$scope.isMouseDown = false;
	};

	switch ($scope.type) {
	case "viewer":
		// TODO: fetch information.
		$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
		break;
	case "session":
		$http.get('/api/sessioninfo/' + $scope.sessionId).
		success(function(data, status, header, config) {
			$log.log('session info: ', data);
			$scope.title = data.title;
			$scope.id = data.upload_id;
			$scope.owner = data.owner;
			$scope.cmds = data.cmds || [ ];
			$scope.ended = data.ended;
			if (data.page) {
				$scope.pageNum = data.page;
			}
			$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
			var proto = (window.location.protocol == "https:" ? "wss:" : "ws:");
			$scope.wsURL = proto + "//" + window.location.host + "/api/ws";
			$log.log('Opening WebSocket to ' + $scope.wsURL);
			$scope.ws = new WebSocket($scope.wsURL);
			if ($scope.owner) {
				$scope.bindCanvas();
				$log.log('setting onopen to openWebSocketMaster');
				$scope.ws.onopen = $scope.openWebSocketMaster;
			} else {
				$log.log('setting onmessage to onMessageSlave');
				$scope.ws.onopen = $scope.openWebSocketSlave;
				$scope.ws.onmessage = $scope.onMessageSlave;
			}
			$scope.ws.onclose = $scope.reconnectWebsocket;
			$scope.ws.onerror = $scope.logWebsocketError;
		});
		break;
	}
}]);

satsumaApp.controller('LoginCtrl', [ '$scope', '$http', '$rootScope', '$location', '$log', function($scope, $http, $rootScope, $location, $log) {
	$log.log('LoginCtrl: new instance');
	$rootScope.checkedLoggedIn = false;
	$rootScope.loggedIn = false;
	$scope.personaLoggedIn = false;

	$scope.reload = function() {
		// this is not really nice because the scope of who's supposed to receive it is very wide, even though
		// we really only want to communicate it to the other controller.
		$rootScope.$broadcast('loggedIn');
	};

	$scope.signinPersona = function() {
		$log.log("signinPersona called");
		navigator.id.request();
	};

	$scope.signoutPersona = function() {
		$log.log("signoutPersona called");
		navigator.id.logout();
	};

	$scope.onLoginPersona = function(assertion) {
		if ($scope.loggedIn || $scope.personaLoggedIn) {
			$log.log("onLoginPersona: already logged in, no need to login again.");
			return;
		}
		$log.log("onLoginPersona called: assertion = ", assertion);
		$http.post('/auth/persona', { 'assertion': assertion }).
		success(function(data, status, headers, config) {
			$rootScope.checkedLoggedIn = true;
			$rootScope.loggedIn = true;
			$scope.personaLoggedIn = true;
			$log.log('LoginCtrl: loggedIn = ' + $rootScope.loggedIn);
			$rootScope.$broadcast('loggedIn');
		}).
		error(function() {
			alert('There was an error logging in through Persona, please try again later.');
		});
	};

	$scope.onLogoutPersona = function() {
		$log.log("onLogoutPersona called");
	};

	navigator.id.watch({
		onlogin: $scope.onLoginPersona,
		onlogout: $scope.onLogoutPersona
	});

	$scope.signOut = function() {
		$location.path('/');
		$http.post('/api/disconnect').
		success(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
		}).
		error(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
			$log.error('disconnect failed: ' + data);
		});
		if ($scope.personaLoggedIn) {
			$scope.personaLoggedIn = false;
			$scope.signoutPersona();
		}
	};

	$http.get('/api/loggedin').
	success(function(data, status, headers, config) {
		$rootScope.checkedLoggedIn = true;
		$rootScope.loggedIn = data.logged_in;
		$log.log('LoginCtrl: loggedIn = ' + $rootScope.loggedIn);
		$rootScope.$broadcast('loggedIn');
	});
}]);

satsumaApp.controller('MainCtrl', ['$scope', '$http', '$rootScope', '$log', '$timeout', function($scope, $http, $rootScope, $log, $timeout) {
	$log.log('MainCtrl: new instance');

	$scope.error = null;
	$scope.uploads = [ ];
	$scope.sessions = [ ];
	$scope.saved_titles = [ ];
	$scope.loading_uploads = false;
	$scope.loading_sessions = false;
	$scope.get_upload_retries = 0;

	window.signinCallback = function(authData) {
		$log.log('signinCallback called');
		$scope.error = null;
		if (authData['access_token']) {
			$http.post('/api/connect', authData['code']).
			success(function(data, status, headers, config) {
				$rootScope.loggedIn = true;
				$scope.$broadcast("loggedIn");
			}).
			error(function(data, status, headers, config) {
				$scope.error = "Signing in failed. Please try again later.";
			});
		} else if (authData['error']) {
			if (authData['error'] != "immediate_failed") {
				$scope.error = "Signing in failed (" + authData['error'] + ").";
			}
		}
		$scope.$apply();
	};

	$scope.$on("loggedIn", function() {
		$scope.getUploads();
		$scope.getSessions();
	});

	$scope.$on("reload", function() {
		$scope.getUploads();
		$scope.getSessions();
	});

	$scope.getUploads = function() {
		$scope.loading_uploads = true;
		$http.get('/api/getuploads').
		success(function(data, status, headers, config) {
			$scope.uploads = data;
			// if we encounter an upload that is currently being processed, then
			// we attempt to reload the uploads list every 10 seconds, until all
			// the uploads are processed. We do that a maximum of 100 times or
			// otherwise failing conversions and open browsers might constantly
			// request the uploads list every 10 seconds.
			var progress_count = 0;
			for (var i=0;i<$scope.uploads.length;i++) {
				var upload = $scope.uploads[i];
				if (upload.conversion == 'progress') {
					progress_count++;
				}
			}
			if (progress_count > 0) {
				if ($scope.get_upload_retries < 100) {
					$timeout($scope.getUploads, 10000);
					$scope.get_upload_retries++;
				} else {
					$scope.get_upload_retries = 0;
				}
			}
			$scope.loading_uploads = false;
		}).
		error(function() {
			$scope.loading_uploads = false;
		});
	};

	$scope.deleteUpload = function(uploadID) {
		$http.post('/api/delupload', { 'upload_id': uploadID }).
		success(function(data, status, headers, config) {
			$scope.getUploads();
			$scope.getSessions();
		}).
		error(function() {
			$log.error('deleting upload failed');
		});
	};

	$scope.renameUpload = function(idx) {
		$scope.saved_titles[idx] = $scope.uploads[idx].title;
		$scope.uploads[idx].renaming = true;
	};

	$scope.cancelUploadRename = function(idx) {
		$scope.uploads[idx].title = $scope.saved_titles[idx];
		$scope.uploads[idx].renaming = false;
	};

	$scope.saveUploadRename = function(idx) {
		$http.post('/api/renameupload', { "upload_id": $scope.uploads[idx].id, "new_title": $scope.uploads[idx].title }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
			$scope.uploads[idx].renaming = false;
		}).
		error(function() {
			$log.error('renaming upload failed');
			$scope.uploads[idx].renaming = false;
		});
	};

	$scope.getSessions = function() {
		$scope.loading_sessions = true;
		$http.get('/api/getsessions').
		success(function(data, status, headers, config) {
			$scope.sessions = data;
			$scope.loading_sessions = false;
			for (var i=0;i<$scope.sessions.length;i++) {
				var session = $scope.sessions[i];
				session.started_relative = moment(session.started).fromNow();
				if (session.ended && session.ended !== "") {
					session.ended_relative = moment(session.ended).fromNow();
				}
			}
		}).
		error(function() {
			$scope.loading_sessions = false;
		});
	};

	$scope.startSession = function(uploadID) {
		$http.post('/api/startsession', { "upload_id": uploadID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			$log.error('starting session failed');
		});
	};

	$scope.stopSession = function(sessionID) {
		$http.post('/api/stopsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			$log.error('stopping session failed');
		});
	};

	$scope.deleteSession = function(sessionID) {
		$http.post('/api/delsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			$log.error('stopping session failed');
		});
	};

	$scope.closeError = function() {
		$scope.error = null;
	};


	$scope.uploadComplete = function(content, completed) {
		if (completed) {
			$scope.hideUpload();
			$scope.getUploads();
		}
	};

	$scope.hideUpload = function() {
		$scope.showUpload = false;
		$('#upload_form').get(0).reset();
	};

	$scope.openUpload = function() {
		$scope.showUpload = true;
	};

	if ($rootScope.loggedIn) {
		$log.log('logged in, loading uploads and sessions');
		$scope.getUploads();
		$scope.getSessions();
	}

}]);

satsumaApp.controller('SettingsCtrl', [ '$scope', '$http', '$log', function($scope, $http, $log) {
	$scope.getConnectedAuthAPIs = function() {
		$log.log("Settings: getConnectedAuthAPIs");
		$http.get('/api/connected').
		success(function(data, status, header, config) {
			$log.log("Settings: authenticated APIs: ", data);
			$scope.connected = data;
		});
	};
	$scope.personaConnectButtonClicked = false;

	$scope.getConnectedAuthAPIs();

	$scope.connectToPersona = function() {
		$scope.personaConnectButtonClicked = true;
		navigator.id.request();
	};

	$scope.onLoginPersona = function(assertion) {
		if (!$scope.personaConnectButtonClicked) {
			return;
		}
		$log.log("Settings: onLoginPersona called: assertion = ", assertion);
		$http.post('/auth/persona', { 'assertion': assertion }).
		success(function(data, status, headers, config) {
			// connecting was successful, now fetch list of connected auth APIs again.
			$scope.getConnectedAuthAPIs();
			$scope.personaConnectButtonClicked = false;
		}).
		error(function() {
			alert('There was an error logging in through Persona, please try again later.');
			$scope.personaConnectButtonClicked = false;
		});
	};

	$scope.onLogoutPersona = function() {
		$log.log("Settings: onLogoutPersona called");
	};

	navigator.id.watch({
		onlogin: $scope.onLoginPersona,
		onlogout: $scope.onLogoutPersona
	});

}]);
