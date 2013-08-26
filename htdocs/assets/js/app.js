var satsumaApp = angular.module('satsuma', [ 'ngUpload' ]);


satsumaApp.directive('g+signin', function() {
	return {
		restrict: 'E',
		template: '<span></span>',
		replace: true,
		link: function(scope, element, attrs) {

			// Set class.
			attrs.$set('class', 'g-signin');

			attrs.$set('data-clientid', attrs.clientid + '.apps.googleusercontent.com');

			// Some default values, based on prior versions of this directive
			var defaults = {
				callback: 'signinCallback',
				cookiepolicy: 'single_host_origin',
				requestvisibleactions: 'http://schemas.google.com/AddActivity',
				scope: 'https://www.googleapis.com/auth/plus.login https://www.googleapis.com/auth/userinfo.email',
				width: 'wide'
			};

			// Provide default values if not explicitly set
			angular.forEach(Object.getOwnPropertyNames(defaults), function(propName) {
				if (!attrs.hasOwnProperty('data-' + propName)) {
					attrs.$set('data-' + propName, defaults[propName]);
				}
			});

			// Asynchronously load the G+ SDK.
			(function() {
				var po = document.createElement('script');
				po.type = 'text/javascript';
				po.async = true;
				po.src = 'https://apis.google.com/js/client:plusone.js';
				var s = document.getElementsByTagName('script')[0];
				s.parentNode.insertBefore(po, s);
			})();
		}
	};
});

satsumaApp.config(['$routeProvider', '$locationProvider', function($routeProvider, $locationProvider) {
		// TODO: configure routes
		$locationProvider.html5Mode(true);

		$routeProvider.when('/', { templateUrl: '/assets/partials/main.html', controller: 'MainCtrl' });
		$routeProvider.when('/v/:uploadid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });
		$routeProvider.when('/s/:sessionid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });

		$routeProvider.otherwise({ redirectTo: '/' });
	}
]);

satsumaApp.controller('PDFViewCtrl', [ '$scope', '$routeParams', '$http', function($scope, $routeParams, $http) {
	if ($routeParams.uploadid) {
		$scope.type = "viewer";
		$scope.id = $routeParams.uploadid;
	} else if ($routeParams.sessionid) {
		$scope.sessionId = $routeParams.sessionid;
		$scope.type = "session";
	};
	console.log('PDFViewCtrl: new instance. type = ' + $scope.type);

	$scope.pageNum = 1;
	PDFJS.disableWorker = true;
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

	$scope.loadPDF = function(path) {
		PDFJS.getDocument(path).then(function(_pdfDoc) {
			$scope.scale = $scope.origScale;
			$scope.pdfDoc = _pdfDoc;
			$scope.renderPage($scope.pageNum, function(success) {
				$scope.loadProgress = 0;
				$scope.$apply();
			});
		}, function(message, exception) {
			console.log("PDF load error: " + message);
		}, function(progressData) {
			$scope.loadProgress = (100 * progressData.loaded) / progressData.total;
			$scope.loadProgress = Math.round($scope.loadProgress*100)/100;
			console.log('loadProgress = ' + $scope.loadProgress);
			$scope.$apply();
		});
	};

	$scope.renderPage = function(num, callback) {
		$scope.pdfDoc.getPage(num).then(function(page) {
			var viewport = page.getViewport($scope.scale);
			if ($scope.fullscreen) {
				var new_scale = Math.min($scope.scale * (window.screen.height / viewport.height), $scope.scale * (window.screen.width / viewport.width));
				console.log('scale = ' + $scope.scale);
				console.log('new scale = ' + new_scale);
				viewport = page.getViewport(new_scale);
			}

			var canvas = document.getElementById('slide_canvas');
			var ctx = canvas.getContext('2d');

			console.log(viewport);
			canvas.height = viewport.height;
			canvas.width = viewport.width;

			page.render({ canvasContext: ctx, viewport: viewport }).then(
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
		default:
			console.log('unknown/unimplemented command ' + cmd.cmd);
		}
	};

	$scope.drawLine = function(cmd) {
		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		var xFactor = canvas.width / cmd.canvasWidth;
		var yFactor = canvas.height / cmd.canvasHeight;

		ctx.beginPath();

		ctx.setLineWidth(cmd.width * xFactor);
		ctx.setStrokeColor(cmd.color, 0.5);

		for (var i=0;i<cmd.coords.length;i+=2) {
			ctx.lineTo(cmd.coords[i] * xFactor, cmd.coords[i+1] * yFactor);
		}
		ctx.stroke();
		ctx.closePath();
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
			console.log('fullscreen change: fullscreen = ' + $scope.fullscreen);
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
			console.log('no requestFullscreen function found!');
		}
	};

	$scope.handleKey = function(event) {
		console.log("called handleKey");
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
		console.log('sendCmd: ' + jsonData);
		if ($scope.wsSend) {
			console.log('sendCmd: sending data');
			$scope.ws.send(jsonData);
			$scope.cmds.push(data);
		}
	};

	$scope.openWebSocketMaster = function() {
		console.log('WebSocket: onopen for master called');
		$scope.ws.send(JSON.stringify({"session_id": $scope.sessionId}));
		$scope.wsSend = true;
	};

	$scope.openWebSocketSlave = function() {
		console.log('WebSocket: onopen for slave called');
		$scope.ws.send(JSON.stringify({"session_id": $scope.sessionId}));
	};

	$scope.onMessageSlave = function(evt) {
		console.log('onMessageSlave: received message from server');
		var data = JSON.parse(evt.data);
		$scope.executeCommand(data);
		$scope.cmds.push(data);
	};

	$scope.bindCanvas = function() {
		$('#slide_canvas').mousedown($scope.mouseDown);
		$('#slide_canvas').mousemove($scope.mouseMove);
		$('#slide_canvas').mouseup($scope.mouseUp);
		// TODO: tablet support.
	};

	$scope.relCoords = function(canvas, evt) {
		var rect = canvas.getBoundingClientRect();
		return { x: evt.clientX - rect.left, y: evt.clientY - rect.top };
	};

	$scope.mouseDown = function(e) {
		var canvas = document.getElementById('slide_canvas');
		var ctx = canvas.getContext('2d');

		$scope.isMouseDown = true;

		ctx.setLineWidth($scope.lineWidth);
		ctx.setStrokeColor($scope.lineColor, 0.5);

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

		ctx.setLineWidth($scope.lineWidth);
		ctx.setStrokeColor($scope.lineColor, 0.5);

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

	switch ($scope.type) {
	case "viewer":
		// TODO: fetch information.
		$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
		break;
	case "session":
		$http.get('/api/sessioninfo/' + $scope.sessionId).
		success(function(data, status, header, config) {
			console.log('session info: ', data);
			$scope.title = data.title;
			$scope.id = data.upload_id;
			$scope.owner = data.owner;
			$scope.cmds = data.cmds || [ ];
			if (data.page) {
				$scope.pageNum = data.page;
			}
			$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
			var wsURL = "ws://" + window.location.host + "/api/ws";
			console.log('Opening WebSocket to ' + wsURL);
			$scope.ws = new WebSocket(wsURL);
			if ($scope.owner) {
				$scope.bindCanvas();
				console.log('setting onopen to openWebSocketMaster');
				$scope.ws.onopen = $scope.openWebSocketMaster;
			} else {
				console.log('setting onmessage to onMessageSlave');
				$scope.ws.onopen = $scope.openWebSocketSlave;
				$scope.ws.onmessage = $scope.onMessageSlave;
			}
		});
		break;
	}
}]);

satsumaApp.controller('LoginCtrl', [ '$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	$rootScope.checkedLoggedIn = false;
	$rootScope.loggedIn = false;

	$http.get('/api/loggedin').
	success(function(data, status, headers, config) {
		$rootScope.checkedLoggedIn = true;
		$rootScope.loggedIn = data.logged_in;
	});
}]);

satsumaApp.controller('MainCtrl', ['$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	console.log('MainCtrl: new instance');

	$scope.error = null;
	$scope.uploads = [ ];
	$scope.sessions = [ ];
	$scope.loading_uploads = false;
	$scope.loading_sessions = false;

	window.signinCallback = function(authData) {
		$scope.error = null;
		console.log(authData);
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

	$scope.getUploads = function() {
		$scope.loading_uploads = true;
		$http.get('/api/getuploads').
		success(function(data, status, headers, config) {
			$scope.uploads = data;
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
			console.log('deleting upload failed');
		});
	};

	$scope.getSessions = function() {
		$scope.loading_sessions = true;
		$http.get('/api/getsessions').
		success(function(data, status, headers, config) {
			$scope.sessions = data;
			$scope.loading_sessions = false;
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
			console.log('starting session failed');
		});
	};

	$scope.stopSession = function(sessionID) {
		$http.post('/api/stopsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			console.log('stopping session failed');
		});
	};

	$scope.deleteSession = function(sessionID) {
		$http.post('/api/delsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			console.log('stopping session failed');
		});
	};

	$scope.closeError = function() {
		$scope.error = null;
	};

	$scope.signOut = function() {
		$http.post('/api/disconnect').
		success(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
		}).
		error(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
			console.log('disconnect failed: ' + data);
		});
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
		$scope.getUploads();
		$scope.getSessions();
	}
}]);
